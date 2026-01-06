package session

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"asr_server/config"
	"asr_server/internal/logger"
	"asr_server/internal/pool"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// Session represents a WebSocket session
type Session struct {
	ID          string
	Conn        *websocket.Conn
	VADInstance pool.VADInstanceInterface
	LastSeen    int64
	mu          sync.RWMutex
	closed      int32

	// Context for cancellation propagation
	ctx    context.Context
	cancel context.CancelFunc

	// Send queue and channels
	SendQueue    chan interface{}
	sendDone     chan struct{}
	sendErrCount int32

	// Activity detection
	lastActivity time.Time

	// ten-vad related
	isInSpeech        bool
	currentSegment    []float32
	silenceFrameCount int

	// Configuration reference (for session-specific settings)
	cfg *config.Config
}

// Manager handles session lifecycle and audio processing.
// All configuration is explicitly injected via constructor.
type Manager struct {
	cfg        *config.Config
	sessions   map[string]*Session
	recognizer *sherpa.OfflineRecognizer
	vadPool    pool.VADPoolInterface
	mu         sync.RWMutex

	// Statistics
	totalSessions  int64
	activeSessions int64
	totalMessages  int64

	// Session cleanup
	cleanupTicker  *time.Ticker
	sessionTimeout time.Duration

	// Recognition worker pool to limit concurrent goroutines
	recognitionWorkers    chan struct{}
	maxRecognitionWorkers int

	// Cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

// Default settings for session management
const (
	DefaultSessionTimeout        = 5 * time.Minute
	DefaultMaxRecognitionWorkers = 50
	CleanupInterval              = 30 * time.Second
	// MaxSegmentSamples limits the maximum size of audio segment to prevent memory exhaustion
	// At 16kHz sample rate, 60 seconds = 960000 samples
	MaxSegmentSamples = 960000
)

// Global buffer pool (8KB)
var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 8192)
	},
}

// Global float32 slice pool
var float32Pool = sync.Pool{}

// Global int16 slice pool for TEN-VAD frame processing
var int16Pool = sync.Pool{}

// getFloat32PoolSlice returns a float32 slice from pool or creates new one
func getFloat32PoolSlice(chunkSize int) []float32 {
	if chunkSize <= 0 {
		chunkSize = config.DefaultChunkSize
	}
	return make([]float32, chunkSize)
}

// NewManager creates a new session manager with explicit dependencies
func NewManager(cfg *config.Config, recognizer *sherpa.OfflineRecognizer, vadPool pool.VADPoolInterface) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		cfg:                   cfg,
		sessions:              make(map[string]*Session),
		recognizer:            recognizer,
		vadPool:               vadPool,
		ctx:                   ctx,
		cancel:                cancel,
		sessionTimeout:        DefaultSessionTimeout,
		maxRecognitionWorkers: DefaultMaxRecognitionWorkers,
		recognitionWorkers:    make(chan struct{}, DefaultMaxRecognitionWorkers),
	}

	// Start session cleanup routine
	manager.startCleanupRoutine()

	return manager
}

// startCleanupRoutine starts the background session cleanup goroutine
func (m *Manager) startCleanupRoutine() {
	m.cleanupTicker = time.NewTicker(CleanupInterval)
	go func() {
		for {
			select {
			case <-m.cleanupTicker.C:
				m.cleanupInactiveSessions()
			case <-m.ctx.Done():
				m.cleanupTicker.Stop()
				return
			}
		}
	}()
	logger.Info("session_cleanup_routine_started", "interval", CleanupInterval, "timeout", m.sessionTimeout)
}

// cleanupInactiveSessions removes sessions that have been inactive for too long
func (m *Manager) cleanupInactiveSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UnixNano()
	timeoutNano := int64(m.sessionTimeout)
	cleanedCount := 0

	for id, session := range m.sessions {
		lastSeen := atomic.LoadInt64(&session.LastSeen)
		if now-lastSeen > timeoutNano {
			inactiveDuration := time.Duration(now - lastSeen)
			logger.Warn("session_timeout_cleanup", "session_id", id, "inactive_duration", inactiveDuration)
			m.closeSession(session)
			delete(m.sessions, id)
			atomic.AddInt64(&m.activeSessions, -1)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		logger.Info("session_cleanup_completed", "cleaned_count", cleanedCount, "remaining", len(m.sessions))
	}
}

// submitRecognitionTask submits a recognition task with worker pool limiting
func (m *Manager) submitRecognitionTask(sessionCtx context.Context, samples []float32, sampleRate int, sessionID string) {
	select {
	case m.recognitionWorkers <- struct{}{}:
		go func() {
			defer func() { <-m.recognitionWorkers }()

			// Check if session context is cancelled
			select {
			case <-sessionCtx.Done():
				logger.Debug("recognition_task_cancelled", "session_id", sessionID)
				return
			default:
			}

			stream := sherpa.NewOfflineStream(m.recognizer)
			defer sherpa.DeleteOfflineStream(stream)
			stream.AcceptWaveform(sampleRate, samples)
			m.recognizer.Decode(stream)
			result := stream.GetResult()

			// Check again after decoding
			select {
			case <-sessionCtx.Done():
				logger.Debug("recognition_result_discarded_session_closed", "session_id", sessionID)
				return
			default:
			}

			if result != nil {
				m.handleRecognitionResult(sessionID, result.Text, nil)
			} else {
				m.handleRecognitionResult(sessionID, "", fmt.Errorf("recognition failed"))
			}
		}()
	default:
		logger.Warn("recognition_worker_pool_full", "session_id", sessionID, "max_workers", m.maxRecognitionWorkers)
	}
}

// CreateSession creates a new session
func (m *Manager) CreateSession(sessionID string, conn *websocket.Conn) (*Session, error) {
	if m.vadPool == nil {
		return nil, fmt.Errorf("VAD pool is not initialized")
	}

	// Create session context for cancellation propagation
	sessionCtx, sessionCancel := context.WithCancel(m.ctx)

	session := &Session{
		ID:                sessionID,
		Conn:              conn,
		VADInstance:       nil, // Lazy allocation
		LastSeen:          time.Now().UnixNano(),
		closed:            0,
		ctx:               sessionCtx,
		cancel:            sessionCancel,
		SendQueue:         make(chan interface{}, m.cfg.Session.SendQueueSize),
		sendDone:          make(chan struct{}),
		sendErrCount:      0,
		lastActivity:      time.Now(),
		isInSpeech:        false,
		currentSegment:    nil,
		silenceFrameCount: 0,
		cfg:               m.cfg,
	}

	// Start send goroutine
	go session.sendLoop()

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	atomic.AddInt64(&m.totalSessions, 1)
	atomic.AddInt64(&m.activeSessions, 1)

	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if exists {
		atomic.StoreInt64(&session.LastSeen, time.Now().UnixNano())
	}

	return session, exists
}

// RemoveSession removes a session
func (m *Manager) RemoveSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		m.closeSession(session)
		delete(m.sessions, sessionID)
		atomic.AddInt64(&m.activeSessions, -1)
		logger.Info("session_removed", "session_id", sessionID)
	}
}

// sendLoop handles the send queue for a session
func (s *Session) sendLoop() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("session_send_loop_panicked", "session_id", s.ID, "recover", r)
		}
	}()

	for {
		select {
		case msg := <-s.SendQueue:
			if atomic.LoadInt32(&s.closed) == 1 {
				return
			}

			if err := s.Conn.WriteJSON(msg); err != nil {
				atomic.AddInt32(&s.sendErrCount, 1)
				logger.Error("failed_to_send_message", "session_id", s.ID, "error", err)
				if atomic.LoadInt32(&s.sendErrCount) > int32(s.cfg.Session.MaxSendErrors) {
					logger.Error("too_many_send_errors", "session_id", s.ID, "action", "closing_session")
					atomic.StoreInt32(&s.closed, 1)
					return
				}
			} else {
				atomic.StoreInt32(&s.sendErrCount, 0)
			}
		case <-s.sendDone:
			return
		}
	}
}

// ProcessAudioData processes audio data for a session
func (m *Manager) ProcessAudioData(sessionID string, audioData []byte) error {
	session, exists := m.GetSession(sessionID)
	if !exists {
		logger.Error("session_not_found_on_audio", "session_id", sessionID)
		return fmt.Errorf("session %s not found", sessionID)
	}

	if atomic.LoadInt32(&session.closed) == 1 {
		logger.Error("session_already_closed", "session_id", sessionID)
		return fmt.Errorf("session %s is closed", sessionID)
	}

	// Lazy VAD instance allocation
	if session.VADInstance == nil {
		vadInstance, err := m.vadPool.Get()
		if err != nil {
			logger.Error("failed_to_get_vad_instance", "session_id", sessionID, "error", err)
			return fmt.Errorf("failed to get VAD instance for session %s: %v", sessionID, err)
		}
		session.VADInstance = vadInstance
		logger.Info("session_assigned_vad", "session_id", sessionID, "type", vadInstance.GetType(), "id", vadInstance.GetID())
	}

	// Update session activity
	atomic.StoreInt64(&session.LastSeen, time.Now().UnixNano())
	atomic.AddInt64(&m.totalMessages, 1)

	// Validate input data
	if len(audioData) == 0 {
		logger.Warn("empty_audio_data_received", "session_id", sessionID)
		return fmt.Errorf("empty audio data")
	}

	if len(audioData)%2 != 0 {
		logger.Warn("invalid_audio_length", "session_id", sessionID, "length", len(audioData))
		return fmt.Errorf("invalid audio data length: %d", len(audioData))
	}

	// Convert audio data
	numSamples := len(audioData) / 2
	samples := float32Pool.Get()
	var float32Slice []float32
	if samples == nil {
		float32Slice = getFloat32PoolSlice(m.cfg.Audio.ChunkSize)
	} else {
		float32Slice = samples.([]float32)
	}
	if cap(float32Slice) < numSamples {
		float32Slice = make([]float32, numSamples)
	}
	float32Slice = float32Slice[:numSamples]
	defer float32Pool.Put(float32Slice)

	normalizeFactor := m.cfg.Audio.NormalizeFactor
	for i := 0; i < numSamples; i++ {
		sample := int16(audioData[i*2]) | int16(audioData[i*2+1])<<8
		float32Slice[i] = float32(sample) / normalizeFactor
	}

	logger.Debug("audio_converted", "session_id", sessionID, "bytes", len(audioData), "samples", numSamples)

	// Process based on VAD type
	switch session.VADInstance.GetType() {
	case pool.SILERO_TYPE:
		return m.processSileroVAD(session, sessionID, float32Slice)
	case pool.TEN_VAD_TYPE:
		return m.processTenVAD(session, sessionID, float32Slice)
	default:
		return fmt.Errorf("unsupported VAD type: %s", session.VADInstance.GetType())
	}
}

// processSileroVAD processes audio with Silero VAD
func (m *Manager) processSileroVAD(session *Session, sessionID string, float32Slice []float32) error {
	sileroInstance, ok := session.VADInstance.(*pool.SileroVADInstance)
	if !ok {
		return fmt.Errorf("invalid Silero VAD instance type")
	}

	// VAD detection with timeout
	vadTimeout := time.Duration(m.cfg.Response.Timeout) * time.Second
	vadCtx, vadCancel := context.WithTimeout(context.Background(), vadTimeout)
	defer vadCancel()

	vadDone := make(chan struct{})
	go func() {
		defer close(vadDone)
		sileroInstance.VAD.AcceptWaveform(float32Slice)
	}()

	select {
	case <-vadDone:
		// VAD processing complete
	case <-vadCtx.Done():
		logger.Warn("vad_processing_timeout", "session_id", sessionID)
		return fmt.Errorf("VAD processing timeout")
	}

	// Process speech segments
	segmentCount := 0
	var speechSegments [][]float32
	sampleRate := m.cfg.Audio.SampleRate

	for !sileroInstance.VAD.IsEmpty() {
		segment := sileroInstance.VAD.Front()
		sileroInstance.VAD.Pop()
		segmentCount++

		if segment != nil && len(segment.Samples) > 0 {
			if atomic.LoadInt32(&session.closed) == 1 {
				logger.Warn("session_closed_during_vad", "session_id", sessionID)
				return fmt.Errorf("session %s closed during processing", sessionID)
			}

			if len(segment.Samples) == 0 {
				logger.Warn("speech_segment_no_samples", "session_id", sessionID, "segment_index", segmentCount)
				continue
			}

			duration := float64(len(segment.Samples)) / float64(sampleRate)
			minSpeechDuration := float64(m.cfg.VAD.SileroVAD.MinSpeechDuration)
			if duration < minSpeechDuration {
				logger.Debug("skipping_short_segment", "session_id", sessionID, "segment_index", segmentCount, "duration", duration, "min", minSpeechDuration)
				continue
			}

			maxDuration := float64(m.cfg.VAD.SileroVAD.MaxSpeechDuration)
			if duration > maxDuration {
				logger.Warn("segment_too_long", "session_id", sessionID, "segment_index", segmentCount, "duration", duration, "max", maxDuration)
				maxSamples := int(maxDuration * float64(sampleRate))
				segment.Samples = segment.Samples[:maxSamples]
			}

			speechSegments = append(speechSegments, segment.Samples)
			logger.Debug("collected_segment", "session_id", sessionID, "segment_index", segmentCount, "samples", len(segment.Samples), "duration", duration)
		} else {
			logger.Warn("empty_speech_segment", "session_id", sessionID, "segment_index", segmentCount)
		}
	}

	// Process collected speech segments using worker pool
	for _, samples := range speechSegments {
		m.submitRecognitionTask(session.ctx, samples, sampleRate, sessionID)
	}

	return nil
}

// processTenVAD processes audio with TEN-VAD
func (m *Manager) processTenVAD(session *Session, sessionID string, float32Slice []float32) error {
	tenVADInstance, ok := session.VADInstance.(*pool.TenVADInstance)
	if !ok {
		return fmt.Errorf("invalid TEN-VAD instance type")
	}

	hopSize := m.cfg.VAD.TenVAD.HopSize
	minSpeechFrames := m.cfg.VAD.TenVAD.MinSpeechFrames
	maxSilenceFrames := m.cfg.VAD.TenVAD.MaxSilenceFrames
	sampleRate := m.cfg.Audio.SampleRate

	// Get or create int16 buffer from pool for frame processing
	var int16Buffer []int16
	if pooled := int16Pool.Get(); pooled != nil {
		int16Buffer = pooled.([]int16)
	}

	// Frame processing
	for i := 0; i < len(float32Slice); i += hopSize {
		end := i + hopSize
		if end > len(float32Slice) {
			end = len(float32Slice)
		}
		frame := float32Slice[i:end]

		// Reuse or allocate int16 buffer
		frameLen := len(frame)
		if int16Buffer == nil || cap(int16Buffer) < frameLen {
			int16Buffer = make([]int16, frameLen)
		}
		int16Frame := int16Buffer[:frameLen]
		for j, f := range frame {
			int16Frame[j] = int16(f * 32768)
		}

		_, flag, err := pool.GetInstance().ProcessAudio(tenVADInstance.Handle, int16Frame)
		if err != nil {
			// Return buffer to pool before returning error
			if int16Buffer != nil {
				int16Pool.Put(int16Buffer)
			}
			return fmt.Errorf("TEN-VAD ProcessAudio error: %v", err)
		}

		if flag == 1 {
			if !session.isInSpeech {
				logger.Debug("speech_started", "session_id", sessionID)
				session.isInSpeech = true
				session.currentSegment = make([]float32, 0)
				session.silenceFrameCount = 0
			}
			session.currentSegment = append(session.currentSegment, frame...)
			session.silenceFrameCount = 0

			// Check if segment exceeds maximum length to prevent memory exhaustion
			if len(session.currentSegment) >= MaxSegmentSamples {
				logger.Warn("segment_max_length_exceeded", "session_id", sessionID,
					"samples", len(session.currentSegment), "max", MaxSegmentSamples)
				// Force recognition of current segment
				segmentCopy := make([]float32, len(session.currentSegment))
				copy(segmentCopy, session.currentSegment)
				m.submitRecognitionTask(session.ctx, segmentCopy, sampleRate, sessionID)
				// Reset segment state
				session.currentSegment = make([]float32, 0)
			}
		} else {
			if session.isInSpeech {
				session.silenceFrameCount++
				session.currentSegment = append(session.currentSegment, frame...)
				if session.silenceFrameCount >= maxSilenceFrames {
					frameCount := len(session.currentSegment) / hopSize
					if frameCount >= minSpeechFrames {
						logger.Debug("speech_segment_completed", "session_id", sessionID, "samples", len(session.currentSegment), "frames", frameCount)
						duration := float64(len(session.currentSegment)) / float64(sampleRate)
						logger.Info("asr_segment_stats", "duration", duration, "samples", len(session.currentSegment))
						segmentCopy := make([]float32, len(session.currentSegment))
						copy(segmentCopy, session.currentSegment)
						// Use worker pool for recognition task
						m.submitRecognitionTask(session.ctx, segmentCopy, sampleRate, sessionID)
					} else {
						logger.Debug("speech_segment_too_short", "session_id", sessionID, "frames", frameCount)
					}
					session.isInSpeech = false
					session.silenceFrameCount = 0
					session.currentSegment = nil
				}
			}
		}
	}

	// Return int16 buffer to pool
	if int16Buffer != nil {
		int16Pool.Put(int16Buffer)
	}

	return nil
}

// handleRecognitionResult handles recognition results
func (m *Manager) handleRecognitionResult(sessionID, result string, err error) {
	session, exists := m.GetSession(sessionID)
	if !exists {
		logger.Warn("recognition_session_not_found", "session_id", sessionID)
		return
	}

	if atomic.LoadInt32(&session.closed) == 1 {
		logger.Warn("recognition_session_closed", "session_id", sessionID)
		return
	}

	if err == nil && len(result) > 0 {
		response := map[string]interface{}{
			"type":      "final",
			"text":      result,
			"timestamp": time.Now().UnixMilli(),
		}
		select {
		case session.SendQueue <- response:
			// Log result length instead of content to prevent sensitive data exposure
			logger.Info("recognition_result_queued", "session_id", sessionID, "result_length", len(result))
		default:
			logger.Warn("recognition_result_dropped", "session_id", sessionID)
		}
		return
	}

	if err != nil {
		logger.Error("recognition_error", "session_id", sessionID, "error", err)
	}
}

// closeSession closes a session
func (m *Manager) closeSession(session *Session) {
	if atomic.CompareAndSwapInt32(&session.closed, 0, 1) {
		// Cancel session context to stop any in-progress recognition tasks
		if session.cancel != nil {
			session.cancel()
		}

		close(session.sendDone)
		for len(session.SendQueue) > 0 {
			<-session.SendQueue
		}

		if session.VADInstance != nil && m.vadPool != nil {
			m.vadPool.Put(session.VADInstance)
			session.VADInstance = nil
			logger.Info("vad_instance_returned", "session_id", session.ID)
		}

		if session.Conn != nil {
			session.Conn.Close()
		}
	}
}

// GetStats returns manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var poolStats map[string]interface{}
	if m.vadPool != nil {
		poolStats = m.vadPool.GetStats()
	} else {
		poolStats = map[string]interface{}{"status": "not_initialized"}
	}

	return map[string]interface{}{
		"total_sessions":   atomic.LoadInt64(&m.totalSessions),
		"active_sessions":  atomic.LoadInt64(&m.activeSessions),
		"total_messages":   atomic.LoadInt64(&m.totalMessages),
		"current_sessions": len(m.sessions),
		"pool_stats":       poolStats,
	}
}

// Shutdown shuts down the manager
func (m *Manager) Shutdown() {
	logger.Info("shutting_down_session_manager")

	// Stop cleanup routine
	m.cancel()
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}

	m.mu.Lock()
	for sessionID, session := range m.sessions {
		logger.Info("closing_session", "session_id", sessionID)
		m.closeSession(session)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	logger.Info("session_manager_shutdown_complete")
}
