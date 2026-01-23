package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var testUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// newWSPair creates a connected WebSocket pair for bidirectional communication
// The server conn can be used for reading, client conn for writing (or vice versa)
func newWSPair() (server, client *websocket.Conn, cleanup func()) {
	var serverConn *websocket.Conn
	var mu sync.Mutex
	ready := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		serverConn = conn
		mu.Unlock()
		close(ready)
		// Keep alive - don't read, let tests control the conn
		<-make(chan struct{}) // Block forever until cleanup
	}))

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		srv.Close()
		return nil, nil, func() {}
	}

	select {
	case <-ready:
	case <-time.After(time.Second):
	}

	mu.Lock()
	server = serverConn
	mu.Unlock()

	return server, clientConn, func() {
		if clientConn != nil {
			clientConn.Close()
		}
		mu.Lock()
		if serverConn != nil {
			serverConn.Close()
		}
		mu.Unlock()
		srv.Close()
	}
}

// newWSPairWithEcho creates a pair where server echoes back messages
func newWSPairWithEcho() (server, client *websocket.Conn, cleanup func()) {
	var serverConn *websocket.Conn
	var mu sync.Mutex
	ready := make(chan struct{})
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		mu.Lock()
		serverConn = conn
		mu.Unlock()
		close(ready)
		// Echo loop
		for {
			select {
			case <-done:
				return
			default:
				mt, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				conn.WriteMessage(mt, msg)
			}
		}
	}))

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		srv.Close()
		return nil, nil, func() {}
	}

	select {
	case <-ready:
	case <-time.After(time.Second):
	}

	mu.Lock()
	server = serverConn
	mu.Unlock()

	return server, clientConn, func() {
		close(done)
		if clientConn != nil {
			clientConn.Close()
		}
		mu.Lock()
		if serverConn != nil {
			serverConn.Close()
		}
		mu.Unlock()
		srv.Close()
	}
}
