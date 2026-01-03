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

	// Cleanup
	ctx    context.Context
	cancel context.CancelFunc
}

// Global buffer pool (8KB)
var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 8192)
	},
}

// Global float32 slice pool
var float32Pool = sync.Pool{}

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
		cfg:        cfg,
		sessions:   make(map[string]*Session),
		recognizer: recognizer,
		vadPool:    vadPool,
		ctx:        ctx,
		cancel:     cancel,
	}

	return manager
}

// CreateSession creates a new session
func (m *Manager) CreateSession(sessionID string, conn *websocket.Conn) (*Session, error) {
	if m.vadPool == nil {
		return nil, fmt.Errorf("VAD pool is not initialized")
	}

	session := &Session{
		ID:                sessionID,
		Conn:              conn,
		VADInstance:       nil, // Lazy allocation
		LastSeen:          time.Now().UnixNano(),
		closed:            0,
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
		logger.Infof("🗑️  Session removed")
	}
}

// sendLoop handles the send queue for a session
func (s *Session) sendLoop() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("❌ Send loop panicked for session %s: %v", s.ID, r)
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
				logger.Errorf("Failed to send message to session %s: %v", s.ID, err)
				if atomic.LoadInt32(&s.sendErrCount) > int32(s.cfg.Session.MaxSendErrors) {
					logger.Errorf("Too many send errors for session, closing")
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
		logger.Errorf("Session %s not found when processing audio data", sessionID)
		return fmt.Errorf("session %s not found", sessionID)
	}

	if atomic.LoadInt32(&session.closed) == 1 {
		logger.Errorf("Session %s is closed, cannot process audio data", sessionID)
		return fmt.Errorf("session %s is closed", sessionID)
	}

	// Lazy VAD instance allocation
	if session.VADInstance == nil {
		vadInstance, err := m.vadPool.Get()
		if err != nil {
			logger.Errorf("Failed to get VAD instance for session %s: %v", sessionID, err)
			return fmt.Errorf("failed to get VAD instance for session %s: %v", sessionID, err)
		}
		session.VADInstance = vadInstance
		logger.Infof("✅ Session %s assigned %s VAD instance %d", sessionID, vadInstance.GetType(), vadInstance.GetID())
	}

	// Update session activity
	atomic.StoreInt64(&session.LastSeen, time.Now().UnixNano())
	atomic.AddInt64(&m.totalMessages, 1)

	// Validate input data
	if len(audioData) == 0 {
		logger.Warnf("Session %s: Received empty audio data", sessionID)
		return fmt.Errorf("empty audio data")
	}

	if len(audioData)%2 != 0 {
		logger.Warnf("Session %s: Audio data length %d is not even (expecting 16-bit samples)", sessionID, len(audioData))
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

	logger.Debugf("Session %s: Converted %d bytes to %d float32 samples", sessionID, len(audioData), numSamples)

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
		logger.Warnf("Session %s: VAD processing timeout", sessionID)
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
				logger.Warnf("Session %s closed during speech segment processing", sessionID)
				return fmt.Errorf("session %s closed during processing", sessionID)
			}

			if len(segment.Samples) == 0 {
				logger.Warnf("Session %s: Speech segment %d has no samples", sessionID, segmentCount)
				continue
			}

			duration := float64(len(segment.Samples)) / float64(sampleRate)
			minSpeechDuration := float64(m.cfg.VAD.SileroVAD.MinSpeechDuration)
			if duration < minSpeechDuration {
				logger.Debugf("Session %s: Skipping short segment %d (%.2fs < %.2fs)", sessionID, segmentCount, duration, minSpeechDuration)
				continue
			}

			maxDuration := float64(m.cfg.VAD.SileroVAD.MaxSpeechDuration)
			if duration > maxDuration {
				logger.Warnf("Session %s: Segment %d too long (%.2fs > %.2fs), truncating", sessionID, segmentCount, duration, maxDuration)
				maxSamples := int(maxDuration * float64(sampleRate))
				segment.Samples = segment.Samples[:maxSamples]
			}

			speechSegments = append(speechSegments, segment.Samples)
			logger.Debugf("Session %s: Collected segment %d with %d samples (%.2fs)", sessionID, segmentCount, len(segment.Samples), duration)
		} else {
			logger.Warnf("Session %s: Empty or null speech segment %d", sessionID, segmentCount)
		}
	}

	// Process collected speech segments
	for i, samples := range speechSegments {
		taskID := fmt.Sprintf("%s_%d_%d", sessionID, time.Now().UnixNano(), i)
		go func(samples []float32, sampleRate int, sessionID string, taskID string) {
			stream := sherpa.NewOfflineStream(m.recognizer)
			defer sherpa.DeleteOfflineStream(stream)
			stream.AcceptWaveform(sampleRate, samples)
			m.recognizer.Decode(stream)
			result := stream.GetResult()
			if result != nil {
				m.handleRecognitionResult(sessionID, result.Text, nil)
			} else {
				m.handleRecognitionResult(sessionID, "", fmt.Errorf("recognition failed"))
			}
		}(samples, sampleRate, sessionID, taskID)
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

	// Frame processing
	for i := 0; i < len(float32Slice); i += hopSize {
		end := i + hopSize
		if end > len(float32Slice) {
			end = len(float32Slice)
		}
		frame := float32Slice[i:end]
		int16Frame := make([]int16, len(frame))
		for j, f := range frame {
			int16Frame[j] = int16(f * 32768)
		}
		_, flag, err := pool.GetInstance().ProcessAudio(tenVADInstance.Handle, int16Frame)
		if err != nil {
			return fmt.Errorf("TEN-VAD ProcessAudio error: %v", err)
		}

		if flag == 1 {
			if !session.isInSpeech {
				logger.Debugf("Session %s: Speech started", sessionID)
				session.isInSpeech = true
				session.currentSegment = make([]float32, 0)
				session.silenceFrameCount = 0
			}
			session.currentSegment = append(session.currentSegment, frame...)
			session.silenceFrameCount = 0
		} else {
			if session.isInSpeech {
				session.silenceFrameCount++
				session.currentSegment = append(session.currentSegment, frame...)
				if session.silenceFrameCount >= maxSilenceFrames {
					frameCount := len(session.currentSegment) / hopSize
					if frameCount >= minSpeechFrames {
						logger.Debugf("Session %s: Speech segment completed with %d samples (%d frames)", sessionID, len(session.currentSegment), frameCount)
						duration := float64(len(session.currentSegment)) / float64(sampleRate)
						logger.Infof("ASR segment length: %.2fs, samples: %d", duration, len(session.currentSegment))
						taskID := fmt.Sprintf("%s_%d", sessionID, time.Now().UnixNano())
						segmentCopy := make([]float32, len(session.currentSegment))
						copy(segmentCopy, session.currentSegment)
						go func(segment []float32, sr int, sid string, tid string) {
							stream := sherpa.NewOfflineStream(m.recognizer)
							defer sherpa.DeleteOfflineStream(stream)
							stream.AcceptWaveform(sr, segment)
							m.recognizer.Decode(stream)
							result := stream.GetResult()
							if result != nil {
								m.handleRecognitionResult(sid, result.Text, nil)
							} else {
								m.handleRecognitionResult(sid, "", fmt.Errorf("recognition failed"))
							}
						}(segmentCopy, sampleRate, sessionID, taskID)
					} else {
						logger.Debugf("Session %s: Speech segment too short (%d frames), discarding", sessionID, frameCount)
					}
					session.isInSpeech = false
					session.silenceFrameCount = 0
					session.currentSegment = nil
				}
			}
		}
	}

	return nil
}

// handleRecognitionResult handles recognition results
func (m *Manager) handleRecognitionResult(sessionID, result string, err error) {
	session, exists := m.GetSession(sessionID)
	if !exists {
		logger.Warnf("Session %s not found when handling recognition result, session may have been closed", sessionID)
		return
	}

	if atomic.LoadInt32(&session.closed) == 1 {
		logger.Warnf("Session %s is closed when handling recognition result", sessionID)
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
			logger.Infof("Recognition result queued for session %s: %s", sessionID, result)
		default:
			logger.Warnf("Session %s send queue is full, dropping recognition result", sessionID)
		}
		return
	}

	if err != nil {
		logger.Errorf("Recognition error for session %s: %v", sessionID, err)
	}
}

// closeSession closes a session
func (m *Manager) closeSession(session *Session) {
	if atomic.CompareAndSwapInt32(&session.closed, 0, 1) {
		close(session.sendDone)
		for len(session.SendQueue) > 0 {
			<-session.SendQueue
		}

		if session.VADInstance != nil && m.vadPool != nil {
			m.vadPool.Put(session.VADInstance)
			session.VADInstance = nil
			logger.Infof("🔄 Returned VAD instance to pool for session %s", session.ID)
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
	logger.Infof("🛑 Shutting down session manager...")

	m.cancel()

	m.mu.Lock()
	for sessionID, session := range m.sessions {
		logger.Infof("🛑 Closing session: %s", sessionID)
		m.closeSession(session)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	logger.Infof("✅ Session manager shutdown complete")
}
