package models

import "time"

type Conversation struct {
    ID        int
    User1ID   int
    User2ID   int
    CreatedAt time.Time
}

type Message struct {
    ID             int
    ConversationID int
    SenderID       int
    SenderUsername string
    Content        string
    CreatedAt      time.Time
}