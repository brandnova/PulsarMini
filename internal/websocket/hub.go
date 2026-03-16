package websocket

import "log"

// Hub maintains all active WebSocket connections and routes pulses
type Hub struct {
    // All connected clients, keyed by userID
    Clients map[int][]*Client

    // Register a new client
    Register chan *Client

    // Unregister a client
    Unregister chan *Client

    // Broadcast a raw message to a specific user
    Broadcast chan UserMessage
}

// UserMessage carries a payload destined for a specific user
type UserMessage struct {
    UserID  int
    Payload []byte
}

func NewHub() *Hub {
    return &Hub{
        Clients:    make(map[int][]*Client),
        Register:   make(chan *Client),
        Unregister: make(chan *Client),
        Broadcast:  make(chan UserMessage, 256),
    }
}

// Run is the hub's main goroutine — it serialises all client map access
// so we never have concurrent map writes (a common Go gotcha)
func (h *Hub) Run() {
    for {
        select {

        case client := <-h.Register:
            h.Clients[client.UserID] = append(h.Clients[client.UserID], client)
            log.Printf("hub: user %d connected (%d tabs)", client.UserID, len(h.Clients[client.UserID]))

        case client := <-h.Unregister:
            clients := h.Clients[client.UserID]
            for i, c := range clients {
                if c == client {
                    h.Clients[client.UserID] = append(clients[:i], clients[i+1:]...)
                    close(client.Send)
                    break
                }
            }
            if len(h.Clients[client.UserID]) == 0 {
                delete(h.Clients, client.UserID)
            }
            log.Printf("hub: user %d disconnected", client.UserID)

        case msg := <-h.Broadcast:
            for _, client := range h.Clients[msg.UserID] {
                select {
                case client.Send <- msg.Payload:
                default:
                    // Send buffer full — drop and clean up
                    close(client.Send)
                    delete(h.Clients, client.UserID)
                }
            }
        }
    }
}