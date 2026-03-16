package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"pulsarmini/internal/auth"
	"pulsarmini/internal/chat"
	"pulsarmini/internal/db"
	"pulsarmini/internal/friends"
	"pulsarmini/internal/pulse"
	ws "pulsarmini/internal/websocket"
)

func main() {
	// ── Database ───────────────────────────────────────────────────
	database, err := db.Init()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatal("Migration failed:", err)
	}

	// ── Redis ──────────────────────────────────────────────────────
	rdb := db.InitRedis()

	// ── WebSocket hub ──────────────────────────────────────────────
	hub := ws.NewHub()
	go hub.Run()

	// ── Pulse system ───────────────────────────────────────────────
	go pulse.Subscribe(context.Background(), rdb, hub)
	pulse.StartClock(context.Background(), database, rdb, hub)

	// ── Router ────────────────────────────────────────────────────
	r := mux.NewRouter()

	// Auth
	authHandler := auth.NewHandler(database)
	r.HandleFunc("/", authHandler.Index).Methods("GET")
	r.HandleFunc("/register", authHandler.RegisterPage).Methods("GET")
	r.HandleFunc("/register", authHandler.Register).Methods("POST")
	r.HandleFunc("/login", authHandler.LoginPage).Methods("GET")
	r.HandleFunc("/login", authHandler.Login).Methods("POST")
	r.HandleFunc("/logout", authHandler.Logout).Methods("POST")
	r.HandleFunc("/dashboard", authHandler.Dashboard).Methods("GET")
	r.HandleFunc("/profile", authHandler.OwnProfile).Methods("GET")
	r.HandleFunc("/profile/update", authHandler.UpdateProfile).Methods("POST")
	r.HandleFunc("/profile/view", authHandler.ViewProfile).Methods("GET")

	// Friends
	friendsHandler := friends.NewHandler(database, rdb, hub)
	r.HandleFunc("/friends", friendsHandler.FriendsList).Methods("GET")
	r.HandleFunc("/friends/request", friendsHandler.SendRequest).Methods("POST")
	r.HandleFunc("/friends/accept", friendsHandler.AcceptRequest).Methods("POST")
	r.HandleFunc("/friends/reject", friendsHandler.RejectRequest).Methods("POST")
	r.HandleFunc("/friends/pending-partial", friendsHandler.PendingPartial).Methods("GET")
	r.HandleFunc("/friends/sidebar-partial", friendsHandler.SidebarPartial).Methods("GET")

	// Chat
	chatHandler := chat.NewHandler(database, rdb, hub)
	r.HandleFunc("/chat/pulse", chatHandler.QueuePulseMessage).Methods("POST")
	r.HandleFunc("/chat/send", chatHandler.SendMessage).Methods("POST")
	r.HandleFunc("/chat/{username}", chatHandler.ChatPage).Methods("GET")

	// WebSocket
	wsHandler := ws.NewHandler(hub)
	r.HandleFunc("/ws", wsHandler.ServeWS)

	// Static files
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	// ── Start server ───────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server running on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}