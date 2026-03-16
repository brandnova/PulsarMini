# Mini Realtime Chat – Project Plan (PulsarMini)

## Project Goal

Build a lightweight realtime chat application using Go that includes:

* user authentication
* friend relationships
* realtime private messaging
* database persistence
* queued message broadcasting (“timely pulses”)

It is a way for me to get to learn Go and how I can use it for realtime projects.
The system will use server-rendered templates with **HTMX** for the UI and **WebSockets** for realtime updates.
Static files will be linked locally (Tailwind, HTMX and so on)
I have redis-cli running locally (and also an online database if needed)

The aim is to use as many of the tools as possible locally first, so that they are compiled by Go.

---

# Core Stack

Backend

* Go
* PostgreSQL (SQLite for dev)
* Redis
* Gorilla WebSocket

Frontend

* HTML templates
* HTMX
* Tailwind and CSS linked via a static folder (I have a tailwind node project that scanns for tailwind classes used in the templates and compiles the respective styling for them into a tailwind.css file in static/css. I use it by running npx tailwindcss -i ./src/styles.css -o ../static/css/tailwind.css --watch in the terminal)

---

# Key Concepts (Borrowed from Pulsar)

The important resemblance to Pulsar will be the **Pulse Event System**.

A **Pulse** is simply an event pushed through Redis and delivered to connected users.

Example pulses:

```
message.created
friend.requested
friend.accepted
user.online
```

The pulse exists as a server-side continous clock that emits occasional pulses (lets say 4 times a day, but shorten the timing so we can test it) the pulses can be used to carry messages that are set on pulse (normal messages can still be sent normally). It can also be used for other things but we plan on keeping it simple for this project.

Flow:

```
User sends a pulse message
      |
Server saves message
      |
Pulse emitted
      |
Redis pub/sub
      |
Websocket server broadcasts
```

This mirrors the event architecture I plan to use in a larger Pulsar chat app.

---

# Core Features (MVP)

### 1 Authentication

Simple session authentication.

Endpoints

```
POST /register
POST /login
POST /logout
```

Database table:

```
users
-----
id
username
email
password_hash
created_at
```

Password hashing should use Go’s standard `bcrypt`.

---

### 2 Friend System

Users must be friends before chatting.

Tables:

```
friend_requests
---------------
id
sender_id
receiver_id
status
created_at
```

```
friends
-------
id
user1_id
user2_id
created_at
```

Endpoints:

```
POST /friends/request
POST /friends/accept
GET  /friends
```

HTMX will update the friend list dynamically.

---

### 3 Chat Conversations

Each friendship allows a conversation.

Tables:

```
conversations
-------------
id
user1_id
user2_id
created_at
```

```
messages
--------
id
conversation_id
sender_id
content
created_at
```

Messages are persisted before being broadcast.

---

### 4 Realtime Messaging

Users connect through WebSockets.

Connection flow:

```
Client loads page
     |
HTMX loads chat UI
     |
Browser opens websocket
     |
Server registers client
```

Each connected user is stored in a **connection hub**.

Example hub structure:

```
Hub
 ├─ connected clients
 ├─ message broadcast channel
 └─ pulse events
```

When a message is saved:

```
message saved
      |
pulse created
      |
redis publish
      |
server receives pulse
      |
broadcast to users
```

---

# Minimal Project Structure (Only a suggestion)

Keep the Go project clean.

```
pulsarmini/
│
├── cmd/
│   server/
│   main.go
│
├── internal/
│
│   ├── auth/
│   │   handlers.go
│   │   service.go
│
│   ├── friends/
│   │   handlers.go
│   │   service.go
│
│   ├── chat/
│   │   handlers.go
│   │   service.go
│
│   ├── websocket/
│   │   hub.go
│   │   client.go
│
│   ├── pulse/
│   │   publisher.go
│   │   subscriber.go
│
│   ├── db/
│   │   postgres.go
│
│   └── models/
│       user.go
│       message.go
│       friendship.go
│
├── templates/
│
│   layout.html
│   login.html
│   dashboard.html
│   chat.html
│
└── static/
    htmx.js
```

The `internal` folder keeps packages private to the project.

---

# Message Pulse Example

When a message is set as a pulse:

```
POST /chat/send
```

Server flow:

```
1 save message
2 publish pulse
3 websocket broadcast
```

Pulse example:

```
{
  type: "message.created",
  conversation_id: 22,
  sender_id: 5,
  content: "hello"
}
```

---

# Realtime Hub Design

Basic structure:

```
hub
 ├─ register channel
 ├─ unregister channel
 ├─ broadcast channel
 └─ clients map
```

Clients listen for pulses.

When a pulse arrives:

```
redis subscriber
       |
hub.broadcast
       |
clients receive message
```

Classic Go concurrency pattern.

---

# UI Flow (HTMX)

Pages:

```
/login
/dashboard
/chat/{friend}
```

HTMX will handle:

```
friend requests
chat message submission
friend list refresh
```

WebSockets handle **incoming messages**.

---

# Development Phases

## Phase 1

Core system.

* user authentication
* Database integration
* HTML templates
* login and dashboard

Goal: users can log in.

---

## Phase 2

Friend system.

* send friend request
* accept request
* list friends

Goal: users have social graph.

---

## Phase 3

Chat persistence.

* create conversations
* send messages
* store messages

Goal: basic messaging works.

---

## Phase 4

Realtime engine.

* websocket hub
* Redis pub/sub
* Users who are friends chat freely
* pulse events
* message broadcast

Goal: realtime chat works.

---

## Phase 5

UI polish.

* HTMX interactions
* friend list updates
* chat interface improvements

Goal: usable mini chat application.

---

# What This Project Will Teach Me

This tiny project quietly forces me to learn the important Go patterns:

* goroutines
* channels
* websocket lifecycle
* message broadcasting
* Redis pub/sub
* event-driven design

Those are **exactly the skills needed** for the Pulsar realtime core.

So the “mini project” is actually a **training ground**.