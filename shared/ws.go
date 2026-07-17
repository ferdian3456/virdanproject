package shared

import (
	"context"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/google/uuid"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second
	maxMessageSize = 8 * 1024
	sendBuffer     = 256
	maxConnPerUser = 5
)

type WsEvent struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type WsClient struct {
	hub       *WsHub
	conn      *websocket.Conn
	userId    string
	connId    string
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
	onInbound func(userId string, raw []byte)
}

func (client *WsClient) close() {
	client.closeOnce.Do(func() { close(client.done) })
}

func (client *WsClient) writePump() {
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

func (client *WsClient) readPump() {
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

type WsHub struct {
	mu      sync.RWMutex
	clients map[string]map[string]*WsClient
}

func NewWsHub() *WsHub {
	return &WsHub{clients: make(map[string]map[string]*WsClient)}
}

func (hub *WsHub) Register(client *WsClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	conns := hub.clients[client.userId]
	if conns == nil {
		conns = make(map[string]*WsClient)
		hub.clients[client.userId] = conns
	}
	conns[client.connId] = client
}

func (hub *WsHub) Unregister(client *WsClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	conns := hub.clients[client.userId]
	if conns == nil {
		return
	}
	if existing, ok := conns[client.connId]; ok && existing == client {
		delete(conns, client.connId)
		client.close()
		if len(conns) == 0 {
			delete(hub.clients, client.userId)
		}
	}
}

func (hub *WsHub) IsOnline(userId string) bool {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	return len(hub.clients[userId]) > 0
}

func (hub *WsHub) CountConnections(userId string) int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	return len(hub.clients[userId])
}

func (hub *WsHub) ActiveConnectionCount() int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	total := 0
	for _, conns := range hub.clients {
		total += len(conns)
	}
	return total
}

func (hub *WsHub) CloseUser(userId string) {
	hub.mu.Lock()
	conns := hub.clients[userId]
	delete(hub.clients, userId)
	hub.mu.Unlock()
	for _, client := range conns {
		client.close()
	}
}

func (hub *WsHub) deliver(userId string, payload []byte) {
	hub.mu.RLock()
	conns := make([]*WsClient, 0, len(hub.clients[userId]))
	for _, client := range hub.clients[userId] {
		conns = append(conns, client)
	}
	hub.mu.RUnlock()
	for _, client := range conns {
		select {
		case client.send <- payload:
		default:
			hub.Unregister(client)
		}
	}
}

func (hub *WsHub) Serve(conn *websocket.Conn, userId string, onInbound func(userId string, raw []byte), onPresence func(userId string, online bool)) {
	if hub.CountConnections(userId) >= maxConnPerUser {
		_ = conn.WriteJSON(WsEvent{Type: "error", Payload: map[string]string{
			"code": "WS_CONN_LIMIT", "message": "too many connections",
		}})
		_ = conn.Close()
		return
	}
	client := &WsClient{
		hub:       hub,
		conn:      conn,
		userId:    userId,
		connId:    uuid.NewString(),
		send:      make(chan []byte, sendBuffer),
		done:      make(chan struct{}),
		onInbound: onInbound,
	}
	hub.Register(client)
	if onPresence != nil {
		go onPresence(userId, true)
	}
	go client.writePump()
	client.readPump()
	if onPresence != nil && hub.CountConnections(userId) == 0 {
		go onPresence(userId, false)
	}
}

type WsBroker interface {
	Publish(ctx context.Context, targetUserIds []string, ev WsEvent) error
}

type InProcessWsBroker struct {
	hub *WsHub
}

func NewInProcessWsBroker(hub *WsHub) *InProcessWsBroker {
	return &InProcessWsBroker{hub: hub}
}

func (broker *InProcessWsBroker) Publish(_ context.Context, targetUserIds []string, ev WsEvent) error {
	payload, err := sonic.Marshal(ev)
	if err != nil {
		return err
	}
	for _, uid := range targetUserIds {
		broker.hub.deliver(uid, payload)
	}
	return nil
}
