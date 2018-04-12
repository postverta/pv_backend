package util

import (
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

// This is a wrapper of the gorilla websocket connection. It will send ping
// periodically to check connectivity, and also respond to client side's ping
// message or special "_ping" message if the client is a browser.
type KeepAliveWsConn struct {
	Conn *websocket.Conn

	// Interval between two pings sent by the server
	PingInterval time.Duration

	// Timeout waiting for the pong after a ping is sent
	PongTimeout time.Duration

	// Channel for connection interruption
	InterruptedChan chan bool

	writeMutex sync.Mutex
	pingTimer  *time.Timer
	pongTimer  *time.Timer
	closeChan  chan bool
}

func NewKeepAliveWsConn(conn *websocket.Conn, pingInterval time.Duration, pongTimeout time.Duration) *KeepAliveWsConn {
	kaConn := &KeepAliveWsConn{
		Conn:            conn,
		PingInterval:    pingInterval,
		PongTimeout:     pongTimeout,
		InterruptedChan: make(chan bool, 1),
		pingTimer:       time.NewTimer(pingInterval),
		pongTimer:       time.NewTimer(pongTimeout),
		closeChan:       make(chan bool, 1),
	}
	kaConn.pongTimer.Stop()

	conn.SetPongHandler(func(appData string) error {
		kaConn.pongTimer.Stop()
		// schedule the next ping
		kaConn.pingTimer.Reset(kaConn.PingInterval)
		return nil
	})

	go func() {
		for {
			select {
			case <-kaConn.pingTimer.C:
				kaConn.writeMutex.Lock()
				err := kaConn.Conn.WriteMessage(websocket.PingMessage, []byte{})
				kaConn.writeMutex.Unlock()
				if err != nil {
					kaConn.InterruptedChan <- true
					return
				}
				kaConn.pongTimer.Reset(kaConn.PongTimeout)
			case <-kaConn.pongTimer.C:
				kaConn.InterruptedChan <- true
				return
			case <-kaConn.closeChan:
				return
			}
		}
	}()

	return kaConn
}

// The user of this object must call ReadMessage even if no incoming message is
// expected. This will process the ping/pong messages.
func (kaConn *KeepAliveWsConn) ReadMessage() (messageType int, p []byte, err error) {
	for {
		messageType, p, err = kaConn.Conn.ReadMessage()
		if err != nil {
			break
		}

		if messageType == websocket.TextMessage && p[0] == '_' && string(p) == "_ping" {
			// special heartbeat message, respond (ignore error)
			kaConn.writeMutex.Lock()
			kaConn.Conn.WriteMessage(websocket.TextMessage, []byte("_pong"))
			kaConn.writeMutex.Unlock()
		} else {
			break
		}
	}
	return
}

func (kaConn *KeepAliveWsConn) WriteMessage(messageType int, data []byte) error {
	kaConn.writeMutex.Lock()
	defer kaConn.writeMutex.Unlock()
	return kaConn.Conn.WriteMessage(messageType, data)
}

func (kaConn *KeepAliveWsConn) Close() error {
	kaConn.closeChan <- true
	return kaConn.Conn.Close()
}
