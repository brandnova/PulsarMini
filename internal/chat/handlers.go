package chat

import (
    "context"
    "database/sql"
    "net/http"
    "os"
    "strconv"

    "github.com/gorilla/mux"
    "github.com/gorilla/sessions"
    "github.com/redis/go-redis/v9"
    "pulsarmini/internal/pulse"
    "pulsarmini/internal/tmpl"
    "pulsarmini/internal/friends"
    ws "pulsarmini/internal/websocket"
    "pulsarmini/internal/db"
)

var store = func() *sessions.CookieStore {
    secret := os.Getenv("SESSION_SECRET")
    if secret == "" {
        secret = "dev-only-insecure-secret"
    }
    return sessions.NewCookieStore([]byte(secret))
}()

var t = tmpl.Load()

type Handler struct {
    DB  *sql.DB
    RDB *redis.Client
    Hub *ws.Hub
}

func NewHandler(db *sql.DB, rdb *redis.Client, hub *ws.Hub) *Handler {
    return &Handler{DB: db, RDB: rdb, Hub: hub}
}

func (h *Handler) ChatPage(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    username, _ := session.Values["username"].(string)
    if !ok {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }

    friendUsername := mux.Vars(r)["username"]
    var friendID int
    err := h.DB.QueryRow(
        db.RebindQuery("SELECT id FROM users WHERE username=?"), 
        friendUsername,
    ).Scan(&friendID)
    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    ok, err = AreFriends(h.DB, userID, friendID)
    if err != nil || !ok {
        http.Error(w, "You are not friends with this user", http.StatusForbidden)
        return
    }

    convID, err := GetOrCreateConversation(h.DB, userID, friendID)
    if err != nil {
        http.Error(w, "Could not load conversation", http.StatusInternalServerError)
        return
    }

    // Clear unread for this user when they open the chat
    ClearUnread(h.DB, userID, convID)

    messages, err := GetMessages(h.DB, convID)
    if err != nil {
        http.Error(w, "Could not load messages", http.StatusInternalServerError)
        return
    }

    msRemaining := pulse.NextPulseIn().Milliseconds()

    sidebar, _ := friends.GetSidebarData(h.DB, userID)

    t.ExecuteTemplate(w, "chat.html", map[string]any{
        "Username":       username,
        "FriendUsername": friendUsername,
        "ConversationID": convID,
        "Messages":       messages,
        "UserID":         userID,
        "FriendID":       friendID,
        "PulseMS":        msRemaining,
        "Sidebar":        sidebar,
    })
}

func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    convID, err := strconv.Atoi(r.FormValue("conversation_id"))
    if err != nil {
        http.Error(w, "Invalid conversation", http.StatusBadRequest)
        return
    }
    receiverID, err := strconv.Atoi(r.FormValue("receiver_id"))
    if err != nil {
        http.Error(w, "Invalid receiver", http.StatusBadRequest)
        return
    }
    content := r.FormValue("content")
    if content == "" {
        w.WriteHeader(http.StatusOK)
        return
    }

    msg, err := SaveMessage(h.DB, convID, userID, content)
    if err != nil {
        http.Error(w, "Could not send message", http.StatusInternalServerError)
        return
    }

    // Increment unread for the receiver
    IncrementUnread(h.DB, receiverID, convID)

    pulse.Publish(context.Background(), h.RDB, pulse.Pulse{
        Type:           "message.created",
        ConversationID: convID,
        SenderID:       userID,
        ReceiverID:     receiverID,
        SenderUsername: msg.SenderUsername,
        Content:        content,
        MessageID:      msg.ID,
    })

    t.ExecuteTemplate(w, "message-bubble", map[string]any{
        "Message": msg,
        "UserID":  userID,
    })
}

// QueuePulseMessage handles POST /chat/pulse — saves a message to be
// delivered on the next pulse tick rather than immediately
func (h *Handler) QueuePulseMessage(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    convID, err := strconv.Atoi(r.FormValue("conversation_id"))
    if err != nil {
        http.Error(w, "Invalid conversation", http.StatusBadRequest)
        return
    }
    receiverID, err := strconv.Atoi(r.FormValue("receiver_id"))
    if err != nil {
        http.Error(w, "Invalid receiver", http.StatusBadRequest)
        return
    }
    content := r.FormValue("content")
    if content == "" {
        w.WriteHeader(http.StatusOK)
        return
    }

    _, err = h.DB.Exec(
        db.RebindQuery(`
            INSERT INTO pulse_messages (conversation_id, sender_id, receiver_id, content)
            VALUES (?, ?, ?, ?)`),
        convID, userID, receiverID, content,
    )
    if err != nil {
        http.Error(w, "Could not queue pulse message", http.StatusInternalServerError)
        return
    }

    t.ExecuteTemplate(w, "pulse-queued", map[string]any{
        "Content":        content,
        "ConversationID": convID,
    })
}