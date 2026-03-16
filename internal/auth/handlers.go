package auth

import (
    "database/sql"
    "net/http"
    "os"

    "github.com/gorilla/sessions"
    "pulsarmini/internal/friends"
    "pulsarmini/internal/tmpl"
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
    DB *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
    return &Handler{DB: db}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    _, loggedIn := session.Values["user_id"].(int)
    t.ExecuteTemplate(w, "index.html", map[string]any{
        "LoggedIn": loggedIn,
    })
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
    t.ExecuteTemplate(w, "register.html", nil)
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
    t.ExecuteTemplate(w, "login.html", nil)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
    username  := r.FormValue("username")
    email     := r.FormValue("email")
    password  := r.FormValue("password")
    firstName := r.FormValue("first_name")
    lastName  := r.FormValue("last_name")

    if err := RegisterUser(h.DB, username, email, password, firstName, lastName); err != nil {
        http.Error(w, "Registration failed: "+err.Error(), http.StatusBadRequest)
        return
    }
    http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
    username := r.FormValue("username")
    password := r.FormValue("password")

    user, err := AuthenticateUser(h.DB, username, password)
    if err != nil {
        http.Error(w, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    session, _ := store.Get(r, "session")
    session.Values["user_id"]  = user.ID
    session.Values["username"] = user.Username
    session.Save(r, w)

    http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    session.Options.MaxAge = -1
    session.Save(r, w)
    http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    user, err := GetUserByID(h.DB, userID)
    if err != nil {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    sidebar, _ := friends.GetSidebarData(h.DB, userID)
    t.ExecuteTemplate(w, "dashboard.html", map[string]any{
        "User":    user,
        "Sidebar": sidebar,
    })
}

// OwnProfile shows the logged-in user's own profile
func (h *Handler) OwnProfile(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    user, err := GetUserByID(h.DB, userID)
    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }
    sidebar, _ := friends.GetSidebarData(h.DB, userID)
    t.ExecuteTemplate(w, "profile.html", map[string]any{
        "ProfileUser": user,
        "IsOwn":       true,
        "Username":    user.Username,
        "Sidebar":     sidebar,
    })
}

// ViewProfile shows another user's profile
func (h *Handler) ViewProfile(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    viewerID, ok := session.Values["user_id"].(int)
    viewerUsername, _ := session.Values["username"].(string)
    if !ok {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }

    // Get username from query param: /profile?u=alice
    targetUsername := r.URL.Query().Get("u")
    if targetUsername == "" || targetUsername == viewerUsername {
        http.Redirect(w, r, "/profile", http.StatusSeeOther)
        return
    }

    profileUser, err := GetUserByUsername(h.DB, targetUsername)
    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }
    sidebar, _ := friends.GetSidebarData(h.DB, viewerID)

    // Check if they are friends
    areFriends := false
    var count int
    h.DB.QueryRow(
        db.RebindQuery(`
            SELECT COUNT(*) FROM friends
            WHERE (user1_id=? AND user2_id=?) OR (user1_id=? AND user2_id=?)`),
        viewerID, profileUser.ID, profileUser.ID, viewerID,
    ).Scan(&count)
    areFriends = count > 0

    t.ExecuteTemplate(w, "profile.html", map[string]any{
        "ProfileUser": profileUser,
        "IsOwn":       false,
        "AreFriends":  areFriends,
        "Username":    viewerUsername,
        "Sidebar":     sidebar,
    })
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session")
    userID, ok := session.Values["user_id"].(int)
    if !ok {
        http.Redirect(w, r, "/login", http.StatusSeeOther)
        return
    }
    firstName := r.FormValue("first_name")
    lastName  := r.FormValue("last_name")
    _, err := h.DB.Exec(
        db.RebindQuery("UPDATE users SET first_name=?, last_name=? WHERE id=?"),
        firstName, lastName, userID,
    )
    if err != nil {
        http.Error(w, "Could not update profile", http.StatusInternalServerError)
        return
    }
    http.Redirect(w, r, "/profile", http.StatusSeeOther)
}