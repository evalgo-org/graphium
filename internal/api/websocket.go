package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"evalgo.org/graphium/internal/storage"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Allow all origins in development
			// TODO: Add proper origin checking in production
			return true
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// WebSocketClient represents a connected WebSocket client.
type WebSocketClient struct {
	conn     *websocket.Conn
	send     chan WebSocketMessage
	server   *Server
	mu       sync.Mutex
	isClosed bool
}

// handleWebSocket handles WebSocket connections for real-time updates.
func (s *Server) handleWebSocket(c echo.Context) error {
	// Upgrade HTTP connection to WebSocket
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	// Create client
	client := &WebSocketClient{
		conn:   ws,
		send:   make(chan WebSocketMessage, 256),
		server: s,
	}

	// Start client goroutines
	go client.readPump()
	go client.writePump()

	// Start watching for changes
	go client.watchChanges()

	return nil
}

// readPump reads messages from the WebSocket connection.
func (c *WebSocketClient) readPump() {
	defer func() {
		c.close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket error: %v\n", err)
			}
			break
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send message as JSON
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// watchChanges listens for storage changes and sends them via WebSocket.
func (c *WebSocketClient) watchChanges() {
	// Container change handler
	containerHandler := func(change storage.ContainerChange) {
		message := WebSocketMessage{
			Type:      "container",
			Action:    string(change.Type),
			Timestamp: time.Now().Format(time.RFC3339),
			Data:      change.Container,
		}

		select {
		case c.send <- message:
		default:
			// Channel full, skip this message
		}
	}

	// Host change handler
	hostHandler := func(change storage.HostChange) {
		message := WebSocketMessage{
			Type:      "host",
			Action:    string(change.Type),
			Timestamp: time.Now().Format(time.RFC3339),
			Data:      change.Host,
		}

		select {
		case c.send <- message:
		default:
			// Channel full, skip this message
		}
	}

	// Start watching all changes
	// Note: This will block until connection closes
	if err := c.server.storage.WatchAllChanges(containerHandler, hostHandler); err != nil {
		fmt.Printf("Error watching changes: %v\n", err)
	}
}

// close closes the WebSocket connection and cleans up resources.
func (c *WebSocketClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return
	}

	c.isClosed = true
	close(c.send)
	c.conn.Close()
}

// broadcastMessage sends a message to a WebSocket client.
func (c *WebSocketClient) broadcastMessage(messageType string, action string, data interface{}) {
	message := WebSocketMessage{
		Type:      messageType,
		Action:    action,
		Timestamp: time.Now().Format(time.RFC3339),
		Data:      data,
	}

	// Marshal to JSON for logging
	if jsonData, err := json.Marshal(message); err == nil {
		fmt.Printf("[WebSocket] %s\n", string(jsonData))
	}

	select {
	case c.send <- message:
	default:
		// Channel full, skip message
	}
}
