package session

import (
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/relay/internal/protocol"
	"github.com/gorilla/websocket"
)

func TestHandleControlRequest_Request(t *testing.T) {
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	server, client, cleanup := newWSPair()
	defer cleanup()
	if server == nil {
		t.Skip("ws failed")
	}
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1", Conn: server}
	s.browsersMu.Unlock()
	req := &protocol.ControlRequest{Action: "request", BrowserID: "b1"}
	payload, _ := protocol.EncodeControlRequest(req)
	msg, _ := protocol.DecodeMessage(payload)
	s.handleControlRequest("b1", msg.Payload)
	client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, data, _ := client.ReadMessage()
	if respMsg, _ := protocol.DecodeMessage(data); respMsg != nil {
		if resp, _ := protocol.DecodeControlRequest(respMsg.Payload); resp != nil && resp.Action == "granted" {
			return
		}
	}
	t.Error("should receive granted")
}

func TestHandleControlRequest_Denied(t *testing.T) {
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	s.controllerMu.Lock()
	s.controllerID = "b0"
	s.controllerMu.Unlock()
	server, client, cleanup := newWSPair()
	defer cleanup()
	if server == nil {
		t.Skip("ws failed")
	}
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1", Conn: server}
	s.browsersMu.Unlock()
	req := &protocol.ControlRequest{Action: "request", BrowserID: "b1"}
	payload, _ := protocol.EncodeControlRequest(req)
	msg, _ := protocol.DecodeMessage(payload)
	s.handleControlRequest("b1", msg.Payload)
	client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, data, _ := client.ReadMessage()
	if respMsg, _ := protocol.DecodeMessage(data); respMsg != nil {
		if resp, _ := protocol.DecodeControlRequest(respMsg.Payload); resp != nil && resp.Action == "denied" {
			return
		}
	}
	t.Error("should receive denied")
}

func TestHandleControlRequest_Release(t *testing.T) {
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	s.controllerMu.Lock()
	s.controllerID = "b1"
	s.controllerMu.Unlock()
	server, client, cleanup := newWSPair()
	defer cleanup()
	if server == nil {
		t.Skip("ws failed")
	}
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1", Conn: server}
	s.browsersMu.Unlock()
	req := &protocol.ControlRequest{Action: "release", BrowserID: "b1"}
	payload, _ := protocol.EncodeControlRequest(req)
	msg, _ := protocol.DecodeMessage(payload)
	s.handleControlRequest("b1", msg.Payload)
	client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, data, _ := client.ReadMessage()
	if respMsg, _ := protocol.DecodeMessage(data); respMsg != nil {
		if resp, _ := protocol.DecodeControlRequest(respMsg.Payload); resp != nil && resp.Action == "released" {
			return
		}
	}
	t.Error("should receive released")
}

func TestHandleControlRequest_Query(t *testing.T) {
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	s.controllerMu.Lock()
	s.controllerID = "b0"
	s.controllerMu.Unlock()
	server, client, cleanup := newWSPair()
	defer cleanup()
	if server == nil {
		t.Skip("ws failed")
	}
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1", Conn: server}
	s.browsersMu.Unlock()
	req := &protocol.ControlRequest{Action: "query", BrowserID: "b1"}
	payload, _ := protocol.EncodeControlRequest(req)
	msg, _ := protocol.DecodeMessage(payload)
	s.handleControlRequest("b1", msg.Payload)
	client.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, data, _ := client.ReadMessage()
	if respMsg, _ := protocol.DecodeMessage(data); respMsg != nil {
		if resp, _ := protocol.DecodeControlRequest(respMsg.Payload); resp != nil && resp.Action == "status" {
			return
		}
	}
	t.Error("should receive status")
}

func TestHandleControlRequest_InvalidPayload(t *testing.T) {
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	server, _, cleanup := newWSPair()
	defer cleanup()
	if server == nil {
		t.Skip("ws failed")
	}
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1", Conn: server}
	s.browsersMu.Unlock()
	s.handleControlRequest("b1", []byte("invalid"))
}

func TestForwardBrowserToRunner_ControlRequest(t *testing.T) {
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	bs, bc, cleanup := newWSPair()
	defer cleanup()
	if bs == nil {
		t.Skip("ws failed")
	}
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1", Conn: bs}
	s.browsersMu.Unlock()
	go s.forwardBrowserToRunner("b1")
	req := &protocol.ControlRequest{Action: "request", BrowserID: "b1"}
	payload, _ := protocol.EncodeControlRequest(req)
	bc.WriteMessage(websocket.BinaryMessage, payload)
	bc.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	bc.ReadMessage()
}

func TestForwardBrowserToRunner_InputNoControl(t *testing.T) {
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	s.controllerMu.Lock()
	s.controllerID = "other"
	s.controllerMu.Unlock()
	rs, rc, c1 := newWSPair()
	defer c1()
	bs, bc, c2 := newWSPair()
	defer c2()
	if rs == nil || bs == nil {
		t.Skip("ws failed")
	}
	s.runnerMu.Lock()
	s.runnerConn = rs
	s.runnerMu.Unlock()
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1", Conn: bs}
	s.browsersMu.Unlock()
	go s.forwardBrowserToRunner("b1")
	bc.WriteMessage(websocket.BinaryMessage, protocol.EncodeInput([]byte("test")))
	rc.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	rc.ReadMessage() // Should timeout
}

func TestForwardRunnerToBrowsers(t *testing.T) {
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	rs, rc, c1 := newWSPair()
	defer c1()
	bs, bc, c2 := newWSPair()
	defer c2()
	if rs == nil || bs == nil {
		t.Skip("ws failed")
	}
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1", Conn: bs}
	s.browsersMu.Unlock()
	s.runnerMu.Lock()
	s.runnerConn = rs
	s.runnerMu.Unlock()
	done := make(chan struct{})
	go func() { s.forwardRunnerToBrowsers(); close(done) }()
	rc.WriteMessage(websocket.BinaryMessage, protocol.EncodeOutput([]byte("hi")))
	bc.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, data, _ := bc.ReadMessage()
	rc.Close()
	<-done
	if msg, _ := protocol.DecodeMessage(data); msg == nil || msg.Type != protocol.MsgTypeOutput {
		t.Error("browser should receive output")
	}
}
