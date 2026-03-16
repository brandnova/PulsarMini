package friends

import (
    "context"
    "database/sql"
    "encoding/json"
    "net/http"
    "os"
    "strconv"

    "github.com/gorilla/sessions"
    "github.com/redis/go-redis/v9"
    "pulsarmini/internal/pulse"
    "pulsarmini/internal/tmpl"
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

func (h *Handler) SendRequest(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    senderUsername, _ := session.Values["username"].(string)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    receiverUsername := r.FormValue("username")
    err := SendRequest(h.DB, userID, receiverUsername)

    pending, _ := GetPendingRequests(h.DB, userID)
    data := map[string]any{
        "Sidebar": SidebarData{Pending: pending},
    }
    if err != nil {
        data["Toast"]   = err.Error()
        data["ToastOK"] = false
    } else {
        data["Toast"]   = "Request sent to " + receiverUsername
        data["ToastOK"] = true

        var receiverID int
        h.DB.QueryRow(
            db.RebindQuery("SELECT id FROM users WHERE username=?"), 
            receiverUsername,
        ).Scan(&receiverID)
        
        var reqID int
        h.DB.QueryRow(
            db.RebindQuery(`
                SELECT id FROM friend_requests
                WHERE sender_id=? AND receiver_id=? AND status='pending'
                ORDER BY created_at DESC LIMIT 1`), 
            userID, receiverID,
        ).Scan(&reqID)

        pulse.Publish(context.Background(), h.RDB, pulse.Pulse{
            Type:           "friend.requested",
            SenderID:       userID,
            ReceiverID:     receiverID,
            FriendUsername: senderUsername,
            RequestID:      reqID,
        })
    }

    t.ExecuteTemplate(w, "pending-section", data)
}

func (h *Handler) AcceptRequest(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    acceptorUsername, _ := session.Values["username"].(string)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    requestID, err := strconv.Atoi(r.FormValue("request_id"))
    if err != nil {
        http.Error(w, "Invalid request ID", http.StatusBadRequest)
        return
    }

    // Get sender ID before accepting
    var senderID int
    h.DB.QueryRow(
        db.RebindQuery("SELECT sender_id FROM friend_requests WHERE id=? AND receiver_id=?"),
        requestID, userID,
    ).Scan(&senderID)

    if err := AcceptRequest(h.DB, requestID, userID); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Notify the original sender that their request was accepted
    pulse.Publish(context.Background(), h.RDB, pulse.Pulse{
        Type:           "friend.accepted",
        SenderID:       userID,
        ReceiverID:     senderID,
        FriendUsername: acceptorUsername,
    })

    h.renderFriendsList(w, r, userID)
}

func (h *Handler) RejectRequest(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    requestID, err := strconv.Atoi(r.FormValue("request_id"))
    if err != nil {
        http.Error(w, "Invalid request ID", http.StatusBadRequest)
        return
    }
    if err := RejectRequest(h.DB, requestID, userID); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    pending, _ := GetPendingRequests(h.DB, userID)
    t.ExecuteTemplate(w, "pending-section", map[string]any{
        "Sidebar": SidebarData{Pending: pending},
    })
}

func (h *Handler) FriendsList(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    h.renderFriendsList(w, r, userID)
}

func (h *Handler) PendingRequests(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    h.renderPendingRequests(w, r, userID)
}

func (h *Handler) renderFriendsList(w http.ResponseWriter, r *http.Request, userID int) {
    sidebar, err := GetSidebarData(h.DB, userID)
    if err != nil {
        http.Error(w, "Could not load friends", http.StatusInternalServerError)
        return
    }
    t.ExecuteTemplate(w, "sidebar", map[string]any{
        "Sidebar": sidebar,
    })
}

func (h *Handler) renderPendingRequests(w http.ResponseWriter, r *http.Request, userID int) {
    pending, err := GetPendingRequests(h.DB, userID)
    if err != nil {
        http.Error(w, "Could not load requests", http.StatusInternalServerError)
        return
    }
    t.ExecuteTemplate(w, "pending-section", map[string]any{
        "Sidebar": SidebarData{Pending: pending},
    })
}

// FriendsListJSON returns friend list as JSON for WebSocket-triggered refreshes
func (h *Handler) FriendsListPartial(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    friends, _ := GetFriends(h.DB, userID)
    pending, _ := GetPendingRequests(h.DB, userID)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "friends_count": len(friends),
        "pending_count": len(pending),
    })
}

// PendingPartial renders just the pending-section for WS-triggered refreshes
func (h *Handler) PendingPartial(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    pending, _ := GetPendingRequests(h.DB, userID)
    t.ExecuteTemplate(w, "pending-section", map[string]any{
        "Sidebar": SidebarData{Pending: pending},
    })
}

// SidebarPartial renders the full sidebar for WS-triggered refreshes
func (h *Handler) SidebarPartial(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    sidebar, _ := GetSidebarData(h.DB, userID)
    t.ExecuteTemplate(w, "sidebar", map[string]any{
        "Sidebar": sidebar,
    })
}