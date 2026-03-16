# PulsarMini

A lightweight realtime chat application built with Go. PulsarMini is a training ground for the patterns behind a larger event-driven chat platform — goroutines, channels, WebSockets, Redis pub/sub, and a unique server-side pulse clock that delivers queued messages on a shared timer.

---

## Features

- **User authentication** — register, login, logout with bcrypt password hashing and cookie sessions
- **Friend system** — send, accept, and reject friend requests with realtime notifications
- **Private messaging** — persistent one-on-one conversations with full message history
- **Realtime delivery** — messages arrive instantly via WebSocket without page reload
- **Pulse clock** — a server-side timer that fires N times per day; users can queue messages to ride the next pulse
- **User profiles** — first/last name, view your own profile or a friend's
- **Unread indicators** — new message dots appear on the friend list when a message arrives on the dashboard
- **Collapsible sidebar** — persistent across all pages via a shared template partial

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go |
| Router | gorilla/mux |
| Sessions | gorilla/sessions |
| WebSockets | gorilla/websocket |
| Database | SQLite (dev) / PostgreSQL (prod) |
| Cache / Pub-Sub | Redis |
| Templates | Go `html/template` |
| Frontend | HTMX, Tailwind CSS |

---

## Project Structure

```
pulsarmini/
├── cmd/server/
│   └── main.go                 # Entry point, router, wiring
├── internal/
│   ├── auth/
│   │   ├── handlers.go         # Register, login, logout, profile
│   │   └── service.go          # bcrypt, user queries
│   ├── chat/
│   │   ├── handlers.go         # Chat page, send message, pulse queue
│   │   └── service.go          # Conversation and message queries
│   ├── db/
│   │   ├── postgres.go         # SQLite/PostgreSQL init and migrations
│   │   └── redis.go            # Redis client init
│   ├── friends/
│   │   ├── handlers.go         # Friend request, accept, reject, sidebar partials
│   │   └── service.go          # Friend queries, SidebarData helper
│   ├── models/
│   │   ├── user.go
│   │   ├── message.go
│   │   └── friendship.go
│   ├── pulse/
│   │   ├── clock.go            # Server-side ticker, firePulse dispatcher
│   │   ├── publisher.go        # Redis publish helper, Pulse struct
│   │   └── subscriber.go       # Redis subscribe, WebSocket broadcast router
│   ├── tmpl/
│   │   └── tmpl.go             # Shared template loader with custom funcMap
│   └── websocket/
│       ├── client.go           # ReadPump / WritePump goroutines
│       ├── handlers.go         # HTTP → WebSocket upgrade
│       └── hub.go              # Connection registry, broadcast channel
├── templates/
│   ├── index.html              # Landing page
│   ├── login.html
│   ├── register.html
│   ├── dashboard.html
│   ├── chat.html
│   ├── profile.html
│   ├── sidebar.html            # Shared sidebar partial
│   └── partials.html           # message-bubble, pending-section, pulse-queued
├── static/
│   ├── css/
│   │   └── tailwind.css        # Compiled by the tailwind/ node project
│   ├── js/
│   │   └── htmx.min.js
│   └── img/
│       └── PulseMiniIcon.png
├── tailwind/                   # Node.js Tailwind compilation project
│   ├── src/styles.css
│   └── tailwind.config.js
├── go.mod
├── go.sum
├── gochat.db                   # SQLite database (dev only, gitignored)
├── .gitignore
├── README.md
└── Info.md
```

---

## Getting Started

### Prerequisites

- Go 1.21+
- Redis running locally (`redis-cli ping` should return `PONG`)
- Node.js + npm (for Tailwind CSS compilation)
- GCC (required by the `go-sqlite3` CGO driver)

### Install dependencies

```bash
go mod download
```

### Compile Tailwind CSS

```bash
cd tailwind
npm install
npx tailwindcss -i ./src/styles.css -o ../static/css/tailwind.css --watch
```

Run this in a separate terminal and leave it running while developing.

### Run the server

```bash
go run cmd/server/main.go
```

Visit [http://localhost:8080](http://localhost:8080).

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `SESSION_SECRET` | *(hardcoded, change in prod)* | Cookie session signing key |
| `DATABASE_URL` | *(SQLite file)* | PostgreSQL DSN for production |

> For production, replace the hardcoded secrets and SQLite driver with environment-driven PostgreSQL. See `Info.md` for the full deployment guide.
