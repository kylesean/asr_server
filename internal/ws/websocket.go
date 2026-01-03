package ws

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"asr_server/config"
	"asr_server/internal/logger"
	"asr_server/internal/session"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"

	"github.com/gorilla/websocket"
)

// Handler handles WebSocket connections with explicit dependencies
type Handler struct {
	cfg              *config.Config
	sessionManager   *session.Manager
	globalRecognizer *sherpa.OfflineRecognizer
	upgrader         websocket.Upgrader
}

// NewHandler creates a new WebSocket handler with explicit dependencies
func NewHandler(cfg *config.Config, sessionManager *session.Manager, globalRecognizer *sherpa.OfflineRecognizer) *Handler {
	return &Handler{
		cfg:              cfg,
		sessionManager:   sessionManager,
		globalRecognizer: globalRecognizer,
		upgrader: websocket.Upgrader{
			CheckOrigin:       func(r *http.Request) bool { return true },
			ReadBufferSize:    cfg.Server.WebSocket.ReadBufferSize,
			WriteBufferSize:   cfg.Server.WebSocket.WriteBufferSize,
			EnableCompression: cfg.Server.WebSocket.EnableCompression,
		},
	}
}

// GenerateSessionID generates a unique session ID
func GenerateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// HandleWebSocket handles WebSocket connections
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Errorf("WebSocket upgrade failed: %v", err)
		return
	}

	wsConfig := h.cfg.Server.WebSocket

	if wsConfig.ReadTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(time.Duration(wsConfig.ReadTimeout) * time.Second))
	}

	sessionID := GenerateSessionID()

	// Create session
	sess, err := h.sessionManager.CreateSession(sessionID, conn)
	if err != nil {
		logger.Errorf("Failed to create session, session_id=%s, error=%v", sessionID, err)
		conn.Close()
		return
	}

	defer func() {
		h.sessionManager.RemoveSession(sessionID)
		logger.Infof("WebSocket connection closed, session_id=%s", sessionID)
	}()

	logger.Infof("New WebSocket connection established, session_id=%s", sessionID)

	// Send connection confirmation
	if sess != nil {
		select {
		case sess.SendQueue <- map[string]interface{}{
			"type":       "connection",
			"message":    "WebSocket connected, ready for audio",
			"session_id": sessionID,
		}:
		default:
			logger.Warnf("Session send queue is full, dropping connection confirmation")
		}
	}

	// Process messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			logger.Warnf("WebSocket read error")
			break
		}

		// Refresh read timeout on each message
		if wsConfig.ReadTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(time.Duration(wsConfig.ReadTimeout) * time.Second))
		}

		// Check message size
		if wsConfig.MaxMessageSize > 0 && len(message) > wsConfig.MaxMessageSize {
			logger.Warnf("Message too large, closing connection")
			break
		}

		// Process audio data
		if len(message) > 0 {
			if err := h.sessionManager.ProcessAudioData(sessionID, message); err != nil {
				logger.Errorf("Failed to process audio data, session_id=%s, error=%v", sessionID, err)
				if sess != nil {
					select {
					case sess.SendQueue <- map[string]interface{}{
						"type":    "error",
						"message": err.Error(),
					}:
					default:
						logger.Warnf("Session send queue is full, dropping error message")
					}
				}
			}
		}
	}
}

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.HandleWebSocket(w, r)
}
