# PulsarMini — Project Info

---

## What Has Been Built

PulsarMini is a complete, working realtime chat application. Every major layer is implemented and integrated.

### Authentication
Users register with a first name, last name, username, email, and password. Passwords are hashed with bcrypt. Sessions are managed with signed cookies via gorilla/sessions. Registration includes client-side confirm-password validation.

### Friend System
Users can search for other users by username and send friend requests. Incoming requests appear in the sidebar with Accept and Reject buttons. Accepting a request creates a friendship record and immediately notifies the original sender via WebSocket — no page refresh required.

### Chat
Friends can open a private conversation. Messages are persisted to the database before being broadcast. The sender sees their message immediately via the HTMX response. The receiver sees it arrive in real time via the WebSocket pipeline. Full message history loads on page open.

### The Pulse System
The pulse clock is a server-side `time.Ticker` that fires on a configurable interval (currently 2 minutes for testing; intended to be 6 hours in production for 4 pulses per day). When users queue a pulse message, it is saved to the `pulse_messages` table with `delivered = 0`. When the clock ticks, all undelivered messages are saved as real messages, marked delivered, and published through Redis. The Redis subscriber broadcasts them via WebSocket to both the sender and the receiver simultaneously. All connected browsers see a pulse dot animation at the same moment because the tick event is broadcast to every connected client. The countdown timer on the chat page is seeded from the server so it reflects the actual remaining time regardless of when the page was loaded.

### Realtime Pipeline
```
HTTP POST /chat/send
    │
    ├── SaveMessage (SQLite)
    ├── IncrementUnread (receiver)
    ├── pulse.Publish → Redis pub/sub
    │       │
    │       └── subscriber goroutine
    │               │
    │               └── hub.Broadcast → receiver's Send channel
    │                       │
    │                       └── WritePump → WebSocket → browser
    │
    └── HTMX response → sender's own bubble appended
```

### Profiles
Each user has a profile page at `/profile` (own) and `/profile/view?u=username` (others). Own profile shows an edit form for first and last name. Others' profiles show a Message button (if friends) or an Add Friend button (if not).

### UI
- Shared sidebar partial (`sidebar.html`) rendered server-side and included on every authenticated page with `{{ template "sidebar" . }}`
- Sidebar is collapsible on every page via a toggle button
- Landing page with animated grid background, floating chat bubbles, and app icon
- Dashboard shows profile card and receives realtime toast notifications for new messages, friend requests, and acceptances
- Chat page has separate send and pulse inputs; pulse input is hidden behind a `+` dropdown and slides up when selected

---

## Points for Improvement

### Security
- The session secret key is hardcoded. It must be moved to an environment variable before any public deployment.
- The WebSocket upgrader has `CheckOrigin` returning `true` (allows all origins). This should be restricted to the known domain in production.
- There is no CSRF protection on POST endpoints. gorilla/csrf should be added.
- Passwords have no server-side minimum length check — only client-side. Add validation in `RegisterUser`.
- Rate limiting on `/login` and `/register` would prevent brute-force and spam.

### Database
- The app uses SQLite for development. SQLite does not support multiple concurrent writers well and should be replaced with PostgreSQL for any multi-user production deployment.
- The `ON CONFLICT` upsert syntax used in `IncrementUnread` is SQLite/PostgreSQL compatible, but confirm it works correctly after switching drivers.
- There are no database indexes beyond the primary key. Add indexes on `messages.conversation_id`, `friend_requests.receiver_id`, and `unread_counts.user_id` for performance at scale.
- Migrations are run with `CREATE TABLE IF NOT EXISTS` on every startup. A proper migration tool (e.g., `golang-migrate`) would be safer for production.

### Features
- No avatar / image upload support yet. Profiles show an initial letter only.
- No group conversations — only one-on-one chat.
- No message deletion or editing.
- No read receipts (the receiver knows they have unread messages, but the sender doesn't know if messages were read).
- No typing indicators.
- No search within conversations.
- The pulse interval is a compile-time constant. It could be made configurable via an environment variable or admin UI.
- Pulse messages are global — any queued message from any conversation fires on the same clock. A per-conversation pulse schedule could be more intentional.
- The friend search is by exact username. A fuzzy search endpoint would improve discoverability.

### Reliability
- The Redis subscriber goroutine has no reconnection logic. If Redis drops, the subscriber exits silently. Add a retry loop with exponential backoff.
- The WebSocket hub iterates over `hub.Clients` inside the subscriber goroutine for the `pulse.tick` broadcast. This is a concurrent map read and should be moved inside `hub.Run()` via a channel.
- There is no graceful shutdown. On `SIGINT`/`SIGTERM`, open WebSocket connections are not cleanly closed. Use `signal.NotifyContext` and a shutdown timeout.

### Frontend
- The Tailwind CSS file must be compiled before deployment. The build step is manual. This should be part of a Makefile or CI pipeline.
- No mobile-optimised layout. The sidebar collapse works, but the chat interface needs further work on small screens.
- HTMX errors (non-2xx responses) are shown as raw `http.Error` text. They should render friendly error states within the UI.

---

## Deploying to Leapcell

Leapcell is a pay-as-you-go hosting platform that supports Go, PostgreSQL, and managed Redis. It deploys directly from a GitHub repository with automatic CI/CD on every push.

### Overview of what Leapcell provides
- **Go build and run** — Leapcell builds your Go binary with `go build` and runs it
- **Managed PostgreSQL** — fully hosted, zero maintenance
- **Managed Redis** — serverless Redis compatible with `go-redis/v9`
- **GitOps** — every push to your linked branch triggers a new deploy
- **Custom domains** with automatic SSL

---

### Step 1 — Switch from SQLite to PostgreSQL

SQLite is fine for development but Leapcell's persistent storage is PostgreSQL. Two changes are needed.

**Install the PostgreSQL driver:**
```bash
go get github.com/lib/pq
```

**Update `internal/db/postgres.go`:**
```go
package db

import (
    "database/sql"
    "log"
    "os"

    _ "github.com/lib/pq"
)

func Init() (*sql.DB, error) {
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        // Fall back to SQLite for local dev
        // swap this import for go-sqlite3 in your local build tag if needed
        log.Fatal("DATABASE_URL is not set")
    }
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }
    if err := db.Ping(); err != nil {
        return nil, err
    }
    return db, nil
}
```

**Update `Migrate` in the same file** — replace SQLite-specific syntax:

| SQLite | PostgreSQL |
|---|---|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
| `DATETIME DEFAULT CURRENT_TIMESTAMP` | `TIMESTAMPTZ DEFAULT NOW()` |
| `?` placeholders | `$1, $2, $3` placeholders |
| `ON CONFLICT(...) DO UPDATE` | same syntax — supported |

All query placeholders throughout the codebase (`?`) must be replaced with `$1`, `$2`, etc. for PostgreSQL.

---

### Step 2 — Read secrets from environment variables

Replace every hardcoded secret with `os.Getenv`.

**Session secret** — in every file that declares `store`:
```go
secret := os.Getenv("SESSION_SECRET")
if secret == "" {
    secret = "dev-only-secret-change-me"
}
var store = sessions.NewCookieStore([]byte(secret))
```

**Redis address** — in `internal/db/redis.go`:
```go
addr := os.Getenv("REDIS_ADDR")
if addr == "" {
    addr = "localhost:6379"
}
rdb := redis.NewClient(&redis.Options{Addr: addr})
```

**Port** — in `cmd/server/main.go`:
```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
log.Fatal(http.ListenAndServe(":"+port, r))
```

---

### Step 3 — Compile Tailwind CSS and commit the output

Leapcell builds Go — it does not run Node. The compiled `static/css/tailwind.css` must be committed to the repository before pushing.

In your `.gitignore`, comment out the tailwind.css exclusion line:
```
# static/css/tailwind.css
```

Then build and commit:
```bash
cd tailwind
npx tailwindcss -i ./src/styles.css -o ../static/css/tailwind.css --minify
cd ..
git add static/css/tailwind.css
git commit -m "build: compile tailwind css for deployment"
```

---

### Step 4 — Push to GitHub

```bash
git init
git add .
git commit -m "feat: initial pulsarmini"
git remote add origin https://github.com/brandnova/PulsarMini.git
git push -u origin main
```

---

### Step 5 — Create the Leapcell services

You will create three Leapcell resources: a PostgreSQL database, a Redis instance, and the Go web service.

#### 5a — Create a PostgreSQL database
1. Go to [leapcell.io](https://leapcell.io) and log in
2. Click **New Resource → PostgreSQL**
3. Give it a name (e.g. `pulsarmini-db`) and choose a region
4. Once created, copy the **Connection String** — it looks like `postgres://user:password@host:5432/dbname`

#### 5b — Create a Redis instance
1. Click **New Resource → Redis**
2. Give it a name (e.g. `pulsarmini-redis`) and choose the same region
3. Once created, copy the **Redis URL** — it looks like `redis://default:password@host:6379`

#### 5c — Create the Go web service
1. Click **New Service**
2. Connect your GitHub account if not already connected
3. Select the `pulsarmini` repository and the `main` branch
4. Configure the build settings:

| Field | Value |
|---|---|
| **Build Command** | `go build -o pulsarmini ./cmd/server` |
| **Run Command** | `./pulsarmini` |
| **Port** | `8080` |

---

### Step 6 — Set environment variables

In the service settings, add the following environment variables:

| Key | Value |
|---|---|
| `DATABASE_URL` | The PostgreSQL connection string from Step 5a |
| `REDIS_ADDR` | The Redis host and port from the Redis URL in Step 5b (format: `host:port`) |
| `SESSION_SECRET` | A long random string — generate with `openssl rand -hex 32` |
| `PORT` | `8080` |

> If your Leapcell Redis URL includes a password (e.g. `redis://:password@host:port`), update `internal/db/redis.go` to parse the full URL:
> ```go
> opt, _ := redis.ParseURL(os.Getenv("REDIS_URL"))
> rdb := redis.NewClient(opt)
> ```
> And set `REDIS_URL` instead of `REDIS_ADDR`.

---

### Step 7 — Deploy

Click **Deploy**. Leapcell will:
1. Clone your repository
2. Run `go build -o pulsarmini ./cmd/server`
3. Start `./pulsarmini`
4. Assign a public URL like `pulsarmini-xxxx.leapcell.dev`

The first deploy runs the database migration automatically on startup (`CREATE TABLE IF NOT EXISTS`).

Every subsequent `git push` to `main` triggers a new build and deploy automatically.

---

### Step 8 — Verify WebSockets work

Leapcell supports persistent WebSocket connections. Verify the connection in the browser console:
```
PulsarMini: WebSocket connected
```

If you see repeated reconnects, check that:
- The service is set to **Persistent Server** mode (not serverless), since serverless instances may be suspended mid-connection
- The `CheckOrigin` function in `internal/websocket/handlers.go` allows your Leapcell domain

To restrict to your deployed domain:
```go
CheckOrigin: func(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    return origin == "https://your-app.leapcell.dev"
},
```

---

### Step 9 — Custom domain (optional)

In the service settings, go to **Domain → Add Custom Domain**, enter your domain, and follow the DNS instructions. Leapcell provisions SSL automatically.

---

### Deployment checklist

- [ ] SQLite replaced with PostgreSQL driver
- [ ] All `?` query placeholders replaced with `$1`, `$2`, ...
- [ ] `SESSION_SECRET` read from environment
- [ ] `REDIS_ADDR` / `REDIS_URL` read from environment
- [ ] `PORT` read from environment
- [ ] Tailwind CSS compiled and committed
- [ ] `gochat.db` excluded by `.gitignore`
- [ ] Environment variables set in Leapcell dashboard
- [ ] Service set to Persistent Server mode for WebSocket support
- [ ] `CheckOrigin` restricted to production domain