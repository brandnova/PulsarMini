package websocket

import (
    "net/http"
    "os"

    "github.com/gorilla/sessions"
    "github.com/gorilla/websocket"
)

var store = func() *sessions.CookieStore {
    secret := os.Getenv("SESSION_SECRET")
    if secret == "" {
        secret = "dev-only-insecure-secret"
    }
    return sessions.NewCookieStore([]byte(secret))
}()

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // In production, restrict to your actual domain
        allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
        if allowedOrigin == "" {
            return true // dev — allow all
        }
        return r.Header.Get("Origin") == allowedOrigin
    },
}

type Handler struct {
    Hub *Hub
}

func NewHandler(hub *Hub) *Handler {
    return &Handler{Hub: hub}
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    client := &Client{
        Hub:    h.Hub,
        Conn:   conn,
        Send:   make(chan []byte, 256),
        UserID: userID,
    }

    h.Hub.Register <- client

    // Each client gets two goroutines — one for reading, one for writing
    // This is the standard Gorilla WebSocket pattern
    go client.WritePump()
    go client.ReadPump()
}