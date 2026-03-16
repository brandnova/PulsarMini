package models

import "time"

type FriendRequest struct {
    ID         int
    SenderID   int
    ReceiverID int
    Status     string
    CreatedAt  time.Time
}

type Friend struct {
    ID        int
    UserID    int
    Username  string
    CreatedAt time.Time
}