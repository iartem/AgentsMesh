package server

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/anthropics/agentsmesh/relay/internal/auth"
	"github.com/anthropics/agentsmesh/relay/internal/session"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 64,  // 64KB
	WriteBufferSize: 1024 * 64,  // 64KB
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development, should be restricted in production
		return true
	},
}

// Handler handles WebSocket connections
type Handler struct {
	sessionManager *session.Manager
	tokenValidator *auth.TokenValidator
	logger         *slog.Logger
}

// NewHandler creates a new WebSocket handler
func NewHandler(sessionManager *session.Manager, tokenValidator *auth.TokenValidator) *Handler {
	return &Handler{
		sessionManager: sessionManager,
		tokenValidator: tokenValidator,
		logger:         slog.With("component", "ws_handler"),
	}
}

// HandleRunnerWS handles runner WebSocket connections
// Path: /runner/terminal?token=xxx
// The token contains session_id, pod_key and runner_id for authentication
func (h *Handler) HandleRunnerWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")

	if tokenStr == "" {
		h.logger.Warn("Runner connection missing token")
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := h.tokenValidator.ValidateToken(tokenStr)
	if err != nil {
		h.logger.Warn("Invalid runner token", "error", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Extract session_id and pod_key from token claims
	sessionID := claims.SessionID
	podKey := claims.PodKey

	if sessionID == "" || podKey == "" {
		h.logger.Warn("Runner token missing session_id or pod_key")
		http.Error(w, "invalid token claims", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade runner connection", "error", err)
		return
	}

	h.logger.Info("Runner connecting (authenticated)",
		"session_id", sessionID,
		"pod_key", podKey,
		"runner_id", claims.RunnerID)

	if err := h.sessionManager.HandleRunnerConnect(sessionID, podKey, conn); err != nil {
		h.logger.Error("Failed to handle runner connect", "error", err, "session_id", sessionID)
		conn.Close()
		return
	}
}

// HandleBrowserWS handles browser WebSocket connections
// Path: /browser/terminal?token=xxx&session_id=xxx
func (h *Handler) HandleBrowserWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	sessionID := r.URL.Query().Get("session_id")

	if tokenStr == "" {
		h.logger.Warn("Browser connection missing token")
		http.Error(w, "token required", http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := h.tokenValidator.ValidateToken(tokenStr)
	if err != nil {
		h.logger.Warn("Invalid token", "error", err)
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Use session ID from token if not provided in query
	if sessionID == "" {
		sessionID = claims.SessionID
	}

	if sessionID == "" {
		h.logger.Warn("Browser connection missing session_id")
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	// Verify session ID matches token
	if claims.SessionID != "" && claims.SessionID != sessionID {
		h.logger.Warn("Session ID mismatch", "token_session", claims.SessionID, "query_session", sessionID)
		http.Error(w, "session_id mismatch", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade browser connection", "error", err)
		return
	}

	// Generate browser ID
	browserID := uuid.New().String()

	h.logger.Info("Browser connecting",
		"session_id", sessionID,
		"pod_key", claims.PodKey,
		"browser_id", browserID,
		"user_id", claims.UserID)

	if err := h.sessionManager.HandleBrowserConnect(sessionID, claims.PodKey, browserID, conn); err != nil {
		h.logger.Error("Failed to handle browser connect", "error", err, "session_id", sessionID)

		// Send error message before closing
		if _, ok := err.(*session.MaxBrowsersError); ok {
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "max browsers reached"))
		}
		conn.Close()
		return
	}
}

// HandleHealth handles health check requests
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// HandleStats handles stats requests
func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.sessionManager.Stats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Simple JSON encoding without external dependency
	w.Write([]byte(`{"active_sessions":` + itoa(stats.ActiveSessions) +
		`,"total_browsers":` + itoa(stats.TotalBrowsers) +
		`,"pending_runners":` + itoa(stats.PendingRunners) +
		`,"pending_browsers":` + itoa(stats.PendingBrowsers) + `}`))
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
