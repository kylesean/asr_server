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
		logger.Error("websocket_upgrade_failed", "error", err)
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
		logger.Error("failed_to_create_session", "session_id", sessionID, "error", err)
		conn.Close()
		return
	}

	defer func() {
		h.sessionManager.RemoveSession(sessionID)
		logger.Info("websocket_connection_closed", "session_id", sessionID)
	}()

	logger.Info("websocket_connection_established", "session_id", sessionID)

	// Send connection confirmation
	if sess != nil {
		select {
		case sess.SendQueue <- map[string]interface{}{
			"type":       "connection",
			"message":    "WebSocket connected, ready for audio",
			"session_id": sessionID,
		}:
		default:
			logger.Warn("session_send_queue_full", "session_id", sessionID, "action", "dropped_confirmation")
		}
	}

	// Process messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			logger.Warn("websocket_read_error", "session_id", sessionID)
			break
		}

		// Refresh read timeout on each message
		if wsConfig.ReadTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(time.Duration(wsConfig.ReadTimeout) * time.Second))
		}

		// Check message size
		if wsConfig.MaxMessageSize > 0 && len(message) > wsConfig.MaxMessageSize {
			logger.Warn("websocket_message_too_large", "session_id", sessionID, "size", len(message))
			break
		}

		// Process audio data
		if len(message) > 0 {
			if err := h.sessionManager.ProcessAudioData(sessionID, message); err != nil {
				logger.Error("failed_to_process_audio", "session_id", sessionID, "error", err)
				if sess != nil {
					select {
					case sess.SendQueue <- map[string]interface{}{
						"type":    "error",
						"message": err.Error(),
					}:
					default:
						logger.Warn("session_send_queue_full", "session_id", sessionID, "action", "dropped_error_message")
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
