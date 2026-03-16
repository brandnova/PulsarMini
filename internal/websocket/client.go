package websocket

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the time allowed to read the next pong message from the peer.
	// Must be longer than pingPeriod.
	pongWait = 30 * time.Second

	// pingPeriod is how often we ping the client.
	// Must be less than pongWait. Set to 10s to keep the connection alive
	// through cloud load balancers and proxies that drop idle connections.
	// Leapcell's own WebSocket examples use a 10s ping interval.
	pingPeriod = 10 * time.Second

	maxMessageSize = 4096
)

// Client represents a single connected browser tab.
type Client struct {
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	UserID int
}

// ReadPump listens for messages from the browser.
// Chat input goes via HTTP POST, but we still need this goroutine to:
//   - Detect disconnections (the read will return an error)
//   - Handle pong frames sent in response to our pings
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		// Reset the deadline every time a pong arrives
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Printf("websocket read error: %v", err)
			}
			break
		}
	}
}

// WritePump pushes messages from the Send channel to the browser,
// and sends periodic pings to keep the connection alive through proxies.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("websocket write error: %v", err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				// Ping failed — connection is dead
				return
			}
		}
	}
}