# Deploying PulsarMini to Leapcell

This document is a practical deployment guide written from direct experience deploying this project. It covers every real problem encountered and how each was resolved, so future deploys — and deployments of similar Go + PostgreSQL + Redis projects — go smoothly the first time.

---

## Prerequisites

- A [Leapcell](https://leapcell.io) account
- A GitHub repository with the project pushed to `main`
- Tailwind CSS compiled and committed (Leapcell has no Node.js build step)
- `go build ./...` passing cleanly locally

---

## Overview of Leapcell Resources

For this project you need three Leapcell resources:

| Resource | Purpose |
|---|---|
| **Web Service** | Runs the compiled Go binary |
| **PostgreSQL** | Persistent relational database |
| **Redis** | Pub/sub backbone for the pulse system and WebSocket broadcasting |

Create all three before configuring the service.

---

## Step 1 — Prepare the codebase

### 1a — PostgreSQL driver

Install the PostgreSQL driver alongside the existing SQLite driver:

```bash
go get github.com/lib/pq
```

Both drivers are imported in `internal/db/postgres.go`. The `Init()` function selects the driver based on whether `DATABASE_URL` is set.

### 1b — Run migrations as individual statements

PostgreSQL **does not allow multiple DDL statements in a single `db.Exec` call**. SQLite does, which is why it works locally but fails on first deploy. The `Migrate` function must execute each `CREATE TABLE IF NOT EXISTS` statement individually in a loop:

```go
for _, stmt := range statements {
    if _, err := database.Exec(stmt); err != nil {
        return fmt.Errorf("migration failed: %w\nSQL:\n%s", err, stmt)
    }
}
```

### 1c — Placeholder syntax

SQLite uses `?` for query placeholders. PostgreSQL requires `$1`, `$2`, `$3` etc. The `RebindQuery` function in `internal/db/postgres.go` converts `?` to `$N` at runtime when `DATABASE_URL` is set. Wrap every parameterised query with it:

```go
db.QueryRow(db.RebindQuery("SELECT id FROM users WHERE username = ?"), username)
```

### 1d — Environment variables in the codebase

Replace every hardcoded value with `os.Getenv`:

**Session secret** — in every file that declares `var store`:
```go
var store = func() *sessions.CookieStore {
    secret := os.Getenv("SESSION_SECRET")
    if secret == "" {
        secret = "dev-only-insecure-secret"
    }
    return sessions.NewCookieStore([]byte(secret))
}()
```

**Port** — in `cmd/server/main.go`:
```go
port := os.Getenv("PORT")
if port == "" {
    port = "8080"
}
log.Fatal(http.ListenAndServe(":"+port, r))
```

**WebSocket origin** — in `internal/websocket/handlers.go`:
```go
CheckOrigin: func(r *http.Request) bool {
    allowed := os.Getenv("ALLOWED_ORIGIN")
    if allowed == "" {
        return true // dev
    }
    return r.Header.Get("Origin") == allowed
},
```

### 1e — Compile Tailwind CSS

Leapcell builds Go only. Node.js is not available in the build environment. The compiled CSS must be committed to the repository.

```bash
cd tailwind
npx tailwindcss -i ./src/styles.css -o ../static/css/tailwind.css --minify
cd ..
```

In `.gitignore`, comment out the tailwind.css exclusion so the file is committed:
```
# static/css/tailwind.css
```

Then add and commit:
```bash
git add static/css/tailwind.css
git commit -m "build: compile tailwind css for deployment"
```

### 1f — Verify the build

```bash
go build ./...
```

Fix any import errors before pushing. Common ones: missing `"os"` after adding env var reads, missing `"pulsarmini/internal/db"` after adding `RebindQuery` calls.

---

## Step 2 — Create Leapcell resources

### PostgreSQL

1. Leapcell dashboard → **New Resource → PostgreSQL**
2. Name it (e.g. `pulsarmini-db`), choose a region
3. Once created, copy the **Connection String** from the resource dashboard

The connection string looks like:
```
postgres://username:password@host:5432/dbname
```

If it starts with `postgresql://`, the Go `lib/pq` driver needs it as `postgres://`. The `Init()` function handles this automatically with `strings.Replace`.

### Redis

1. Leapcell dashboard → **New Resource → Redis**
2. Name it (e.g. `pulsarmini-redis`), choose the same region as the database
3. Once created, note the **host**, **port**, and **password** from the resource dashboard

> **Do not copy the Redis URL string.** See the Redis credentials section below for why.

### Web Service

1. Leapcell dashboard → **New Service**
2. Connect your GitHub account if not already connected
3. Select the repository and `main` branch
4. Configure build settings:

| Field | Value |
|---|---|
| **Build Command** | `go build -o pulsarmini ./cmd/server` |
| **Run Command** | `./pulsarmini` |
| **Port** | `8080` |
| **Service Type** | **Persistent Server** — not serverless |

> The service type must be **Persistent Server**. Serverless instances are suspended between requests, which kills WebSocket connections and the pulse clock goroutine. PulsarMini requires a long-running process.

---

## Step 3 — Set environment variables

In the web service settings → **Environment Variables**, add the following.

### Database

| Key | Value |
|---|---|
| `DATABASE_URL` | The PostgreSQL connection string from Step 2 |

### Redis — use discrete variables, not a URL

Leapcell Redis passwords contain special characters (`+`, `/`, `=`, `@`) that are structurally significant in URLs. Every URL parser — including Go's `net/url` and `redis.ParseURL` — misinterprets these characters and corrupts the password, causing `ERR auth failed` even when the credentials are correct.

**The solution is to set the credentials as three separate variables, never as a combined URL:**

| Key | Value | Example |
|---|---|---|
| `REDIS_HOST` | Hostname only, no port | `pulsarmini-redis-xxxx.leapcell.cloud` |
| `REDIS_PORT` | Port only | `6379` |
| `REDIS_PASSWORD` | Raw password, pasted exactly as shown in the Leapcell Redis dashboard — no encoding, no quotes | `Ae000...+/s` |

The `InitRedis()` function in `internal/db/redis.go` reads these three variables directly into `redis.Options` without any URL parsing, so the password reaches the Redis `AUTH` command byte-for-byte as set.

TLS is always enabled for production Redis (`REDIS_HOST` is set) with `InsecureSkipVerify: true`, which is necessary because managed Redis providers use self-signed or provider-signed certificates that cannot be verified against Go's default certificate pool.

### Application

| Key | Value |
|---|---|
| `SESSION_SECRET` | A long random string. Generate with `openssl rand -hex 32` |
| `PORT` | `8080` |
| `ALLOWED_ORIGIN` | Leave blank for now — set after first deploy once you have the URL |

---

## Step 4 — Push and deploy

Leapcell automatically deploys on every push to the linked branch:

```bash
git push origin main
```

Watch the **Logs** tab in the Leapcell dashboard. A successful startup looks like:

```
db: connecting to PostgreSQL
db: migrations complete
redis: connecting to pulsarmini-redis-xxxx.leapcell.cloud:6379 (TLS)
redis: connected
pulse: clock started — firing every 2m0s
pulse: subscriber listening on pulsar:pulses
Server running on http://localhost:8080
```

If the migrations log line is missing, the PostgreSQL connection failed — check `DATABASE_URL`. If Redis fails, check that `REDIS_HOST`, `REDIS_PORT`, and `REDIS_PASSWORD` are all set and that the password was pasted raw without any encoding.

---

## Step 5 — Post-deploy configuration

Once the service is running and you have your Leapcell URL (e.g. `https://pulsarmini-xxxx.leapcell.dev`):

1. Go back to Environment Variables
2. Set `ALLOWED_ORIGIN` to your full Leapcell URL: `https://pulsarmini-xxxx.leapcell.dev`
3. Leapcell will automatically redeploy with the new variable

This locks WebSocket connections to your domain and prevents third-party sites from opening WebSocket connections to your service.

---

## Step 6 — Custom domain (optional)

In the service settings → **Domain → Add Custom Domain**, enter your domain and follow the DNS instructions. Leapcell provisions and renews SSL automatically.

After adding a custom domain, update `ALLOWED_ORIGIN` to the custom domain URL and redeploy.

---

## Errors encountered and how they were resolved

This section documents every error hit during the initial deployment of PulsarMini, in the order they appeared.

### `redis: invalid REDIS_URL: invalid port after host`

**Cause:** The Redis password contained `+` characters. Go's URL parser treated the `+` following the colon as a delimiter rather than part of the password, so it tried to parse the password as a port number.

**Resolution attempt 1 — percent-encode the password in the URL:** Replaced `+` with `%2B` and `/` with `%2F` in the `REDIS_URL` environment variable. This fixed the URL parsing error but caused the next problem.

**Resolution attempt 2 — encode the password in code using `url.PathEscape`:** Added automatic encoding in `InitRedis` before calling `redis.ParseURL`. This also corrupted the password because `PathEscape` encoded characters that were already valid, resulting in `ERR auth failed`.

**Resolution attempt 3 — manual URL string parsing:** Wrote a `parseRedisURL` function that found the last `@` to split credentials from the host, then split credentials on the first `:`. This still failed because the Leapcell deploy was serving a cached binary that hadn't picked up the new file.

**Final resolution:** Replaced the single `REDIS_URL` variable with three discrete variables (`REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`) and constructed `redis.Options` directly without any URL parsing. The raw password string flows directly to the Redis `AUTH` command without touching a URL parser. This approach is immune to all special character problems.

### `redis: connection failed: EOF`

**Cause:** The server required TLS but the client connected without it. The `redis://` scheme in the URL did not enable TLS; the server expected `rediss://` (double-s).

**Resolution:** Set `TLSConfig: &tls.Config{InsecureSkipVerify: true}` on `redis.Options` unconditionally when connecting to a managed Redis host. `InsecureSkipVerify` is required because managed providers use certificates that Go cannot verify without the provider's CA bundle.

### `redis: connection failed: ERR auth failed`

**Cause:** The password was being corrupted by URL encoding or decoding before being sent to Redis `AUTH`. This occurred even when the URL appeared correct.

**Resolution:** Discrete environment variables (see above). When the password never passes through a URL parser, it cannot be corrupted.

### `Migration failed: cannot insert multiple commands into a prepared statement`

**Cause:** The `Migrate` function executed all `CREATE TABLE` statements as a single string in one `db.Exec` call. PostgreSQL rejects multi-statement DDL in a single prepared statement. SQLite accepts it, so this only failed in production.

**Resolution:** Split the migration into a `[]string` slice of individual statements and execute each one in a loop. Any failure now also logs the exact failing SQL for easy diagnosis.

---

## Deployment checklist

- [ ] `go get github.com/lib/pq` installed
- [ ] `Migrate` executes each statement individually
- [ ] All `?` placeholders wrapped with `db.RebindQuery(...)`
- [ ] `SESSION_SECRET` reads from `os.Getenv`
- [ ] `PORT` reads from `os.Getenv`
- [ ] `ALLOWED_ORIGIN` reads from `os.Getenv` in `CheckOrigin`
- [ ] Tailwind CSS compiled with `--minify` and committed
- [ ] `static/css/tailwind.css` not excluded by `.gitignore`
- [ ] `go build ./...` passes cleanly
- [ ] Leapcell service type set to **Persistent Server**
- [ ] `DATABASE_URL` set in Leapcell environment variables
- [ ] `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` set individually (not as a URL)
- [ ] `SESSION_SECRET` set to a strong random value
- [ ] `PORT` set to `8080`
- [ ] Startup log shows `db: migrations complete` and `redis: connected`
- [ ] `ALLOWED_ORIGIN` set to the production URL after first deploy