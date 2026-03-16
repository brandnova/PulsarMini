package pulse

import (
    "context"
    "encoding/json"

    "github.com/redis/go-redis/v9"
)

const Channel = "pulsar:pulses"

type Pulse struct {
    Type           string `json:"type"`
    ConversationID int    `json:"conversation_id,omitempty"`
    SenderID       int    `json:"sender_id,omitempty"`
    ReceiverID     int    `json:"receiver_id,omitempty"`
    Content        string `json:"content,omitempty"`
    SenderUsername string `json:"sender_username,omitempty"`
    MessageID      int    `json:"message_id,omitempty"`
    // Friend notifications
    RequestID      int    `json:"request_id,omitempty"`
    FriendUsername string `json:"friend_username,omitempty"`
}

func Publish(ctx context.Context, rdb *redis.Client, pulse Pulse) error {
    data, err := json.Marshal(pulse)
    if err != nil {
        return err
    }
    return rdb.Publish(ctx, Channel, data).Err()
}