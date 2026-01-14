package runner

import (
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

// SendMessage sends a message on the connection
func (rc *RunnerConnection) SendMessage(msg *RunnerMessage) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.Conn == nil {
		return ErrConnectionClosed
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case rc.Send <- data:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

// Close closes the connection safely (idempotent)
func (rc *RunnerConnection) Close() {
	rc.closeOnce.Do(func() {
		rc.mu.Lock()
		if rc.Conn != nil {
			rc.Conn.Close()
			rc.Conn = nil
		}
		rc.mu.Unlock()

		// Close send channel safely
		close(rc.Send)
	})
}

// WritePump pumps messages from the send channel to the WebSocket
func (rc *RunnerConnection) WritePump() {
	pingInterval := rc.PingInterval
	if pingInterval <= 0 {
		pingInterval = 30 * time.Second // default fallback
	}
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		rc.Close()
	}()

	for {
		select {
		case message, ok := <-rc.Send:
			rc.mu.Lock()
			conn := rc.Conn
			rc.mu.Unlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				slog.Debug("send channel closed, sending close message", "runner_id", rc.RunnerID)
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Parse message to log type and pod_key
			var msg RunnerMessage
			if err := json.Unmarshal(message, &msg); err == nil {
				slog.Debug("writing message to runner websocket",
					"runner_id", rc.RunnerID,
					"type", msg.Type,
					"pod_key", msg.PodKey,
					"size", len(message))
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				slog.Warn("failed to write message to runner websocket",
					"runner_id", rc.RunnerID,
					"error", err)
				return
			}

		case <-ticker.C:
			rc.mu.Lock()
			conn := rc.Conn
			rc.mu.Unlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Warn("failed to send ping to runner",
					"runner_id", rc.RunnerID,
					"error", err)
				return
			}
		}
	}
}
