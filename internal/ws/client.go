package ws

import (
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second
	maxMessageSize = 8 * 1024
	sendBuffer     = 256
	maxConnPerUser = 5
)

// Client = one device connection. Exactly two goroutines run per client:
// writePump (the ONLY writer of conn) and readPump (the ONLY reader).
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	userId    string
	connId    string
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
	onInbound func(userId string, raw []byte)
}

func (client *Client) close() {
	client.closeOnce.Do(func() { close(client.done) })
}

// writePump is the single writer. Producers never touch conn; they push to send.
func (client *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = client.conn.Close()
	}()
	for {
		select {
		case <-client.done:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = client.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		case msg := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump is the single reader. On any error it unregisters + closes.
func (client *Client) readPump() {
	defer client.hub.Unregister(client)
	client.conn.SetReadLimit(maxMessageSize)
	_ = client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error {
		_ = client.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, raw, err := client.conn.ReadMessage()
		if err != nil {
			return
		}
		if client.onInbound != nil {
			client.onInbound(client.userId, raw)
		}
	}
}
