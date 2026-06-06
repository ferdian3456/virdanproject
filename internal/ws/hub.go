package ws

import (
	"sync"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/google/uuid"
)

// Hub holds local WS connections: userId -> connId -> *Client. Safe for concurrent use.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[string]*Client
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string]map[string]*Client)}
}

func (hub *Hub) Register(client *Client) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	conns := hub.clients[client.userId]
	if conns == nil {
		conns = make(map[string]*Client)
		hub.clients[client.userId] = conns
	}
	conns[client.connId] = client
}

func (hub *Hub) Unregister(client *Client) {
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

func (hub *Hub) IsOnline(userId string) bool {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	return len(hub.clients[userId]) > 0
}

func (hub *Hub) CountConnections(userId string) int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	return len(hub.clients[userId])
}

// ActiveConnectionCount returns total WS connections across all users (for metrics).
func (hub *Hub) ActiveConnectionCount() int {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	total := 0
	for _, conns := range hub.clients {
		total += len(conns)
	}
	return total
}

// CloseUser drops ALL connections of a user. Used for single-session enforcement
// (new login kicks the old session) and logout/token revocation.
func (hub *Hub) CloseUser(userId string) {
	hub.mu.Lock()
	conns := hub.clients[userId]
	delete(hub.clients, userId)
	hub.mu.Unlock()
	for _, client := range conns {
		client.close()
	}
}

// deliver pushes payload to every connection of userId. Non-blocking: a full
// buffer means a slow client — drop it (source of truth is Postgres; client
// refetches on reconnect).
func (hub *Hub) deliver(userId string, payload []byte) {
	hub.mu.RLock()
	conns := make([]*Client, 0, len(hub.clients[userId]))
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

// Serve wires a freshly-upgraded conn into the hub and blocks until disconnect.
// Enforces the per-user connection cap. onInbound handles client->server frames.
func (hub *Hub) Serve(conn *websocket.Conn, userId string, onInbound func(userId string, raw []byte)) {
	if hub.CountConnections(userId) >= maxConnPerUser {
		_ = conn.WriteJSON(Event{Type: "error", Payload: map[string]string{
			"code": "WS_CONN_LIMIT", "message": "too many connections",
		}})
		_ = conn.Close()
		return
	}
	client := &Client{
		hub:       hub,
		conn:      conn,
		userId:    userId,
		connId:    uuid.NewString(),
		send:      make(chan []byte, sendBuffer),
		done:      make(chan struct{}),
		onInbound: onInbound,
	}
	hub.Register(client)
	go client.writePump()
	client.readPump() // blocks
}
