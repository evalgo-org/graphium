package api

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// GraphEventType represents the type of graph event
type GraphEventType string

const (
	EventContainerAdded   GraphEventType = "container_added"
	EventContainerUpdated GraphEventType = "container_updated"
	EventContainerRemoved GraphEventType = "container_removed"
	EventHostAdded        GraphEventType = "host_added"
	EventHostUpdated      GraphEventType = "host_updated"
	EventHostRemoved      GraphEventType = "host_removed"
	EventStackAdded       GraphEventType = "stack_added"
	EventStackUpdated     GraphEventType = "stack_updated"
	EventStackRemoved     GraphEventType = "stack_removed"
	EventStackDeploying   GraphEventType = "stack_deploying"
	EventStackDeployed    GraphEventType = "stack_deployed"
	EventStackError       GraphEventType = "stack_error"
	EventGraphRefresh     GraphEventType = "graph_refresh"
)

// GraphEvent represents a change in the graph
type GraphEvent struct {
	Type      GraphEventType `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      interface{}    `json:"data"`
}

// Client represents a WebSocket client connection
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from clients
	broadcast chan []byte

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected (total: %d)", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected (total: %d)", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client is slow or disconnected, remove it
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastEvent sends an event to all connected clients
func (h *Hub) BroadcastEvent(event GraphEvent) error {
	event.Timestamp = time.Now()
	message, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.broadcast <- message
	return nil
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10
)

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck // Deadline errors are handled by ReadMessage
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck // Deadline errors are handled by ReadMessage
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// We don't expect messages from clients for now, just ignore them
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck // Deadline errors are handled by WriteMessage
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{}) //nolint:errcheck // Connection is closing, error can be ignored
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message) //nolint:errcheck // Write errors are handled by Close

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'}) //nolint:errcheck // Write errors are handled by Close
				_, _ = w.Write(<-c.send)     //nolint:errcheck // Write errors are handled by Close
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait)) //nolint:errcheck // Deadline errors are handled by WriteMessage
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
