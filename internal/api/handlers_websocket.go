package api

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now (consider restricting in production)
		return true
	},
}

// HandleWebSocket handles WebSocket connections for graph updates
// @Summary WebSocket endpoint for real-time graph updates
// @Description Establishes a WebSocket connection for receiving real-time graph events
// @Tags websocket
// @Accept json
// @Produce json
// @Success 101 {string} string "Switching Protocols"
// @Router /ws/graph [get]
func (s *Server) HandleWebSocket(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return err
	}

	client := &Client{
		hub:  s.wsHub,
		conn: ws,
		send: make(chan []byte, 256),
	}

	client.hub.register <- client

	// Start client goroutines
	go client.writePump()
	go client.readPump()

	return nil
}

// GetWebSocketStats returns WebSocket connection statistics
// @Summary Get WebSocket statistics
// @Description Returns statistics about WebSocket connections
// @Tags websocket
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /ws/stats [get]
func (s *Server) GetWebSocketStats(c echo.Context) error {
	stats := map[string]interface{}{
		"connected_clients": s.wsHub.ClientCount(),
		"status":            "operational",
	}
	return c.JSON(http.StatusOK, stats)
}
