package pulse

import (
    "context"
    "encoding/json"
    "log"

    "github.com/redis/go-redis/v9"
    ws "pulsarmini/internal/websocket"
)

func Subscribe(ctx context.Context, rdb *redis.Client, hub *ws.Hub) {
    sub := rdb.Subscribe(ctx, Channel)
    ch := sub.Channel()

    log.Println("pulse: subscriber listening on", Channel)

    for msg := range ch {
        var p Pulse
        if err := json.Unmarshal([]byte(msg.Payload), &p); err != nil {
            log.Printf("pulse: bad payload: %v", err)
            continue
        }

        switch p.Type {

        case "message.created":
            // Send JSON — client renders correct bubble based on its own userID
            payload, _ := json.Marshal(map[string]any{
                "type":            "message.created",
                "message_id":      p.MessageID,
                "sender_id":       p.SenderID,
                "sender_username": p.SenderUsername,
                "content":         p.Content,
                "is_pulse":        false,
            })
            hub.Broadcast <- ws.UserMessage{
                UserID:  p.ReceiverID,
                Payload: payload,
            }

        case "pulse.message.created":
            // Send to both — each renders based on their own userID
            payload, _ := json.Marshal(map[string]any{
                "type":            "pulse.message.created",
                "message_id":      p.MessageID,
                "conversation_id": p.ConversationID,
                "sender_id":       p.SenderID,
                "sender_username": p.SenderUsername,
                "content":         p.Content,
                "is_pulse":        true,
            })
            hub.Broadcast <- ws.UserMessage{
                UserID:  p.ReceiverID,
                Payload: payload,
            }
            hub.Broadcast <- ws.UserMessage{
                UserID:  p.SenderID,
                Payload: payload,
            }

        case "friend.requested":
            // Deliver to receiver — they get a real-time notification
            payload, _ := json.Marshal(map[string]any{
                "type":            "friend.requested",
                "sender_username": p.FriendUsername,
                "request_id":      p.RequestID,
            })
            hub.Broadcast <- ws.UserMessage{
                UserID:  p.ReceiverID,
                Payload: payload,
            }

        case "friend.accepted":
            // Deliver to the original sender of the request
            payload, _ := json.Marshal(map[string]any{
                "type":            "friend.accepted",
                "friend_username": p.FriendUsername,
            })
            hub.Broadcast <- ws.UserMessage{
                UserID:  p.ReceiverID,
                Payload: payload,
            }

        case "pulse.tick":
            // Broadcast tick to every connected client
            payload, _ := json.Marshal(map[string]any{
                "type": "pulse.tick",
            })
            for userID := range hub.Clients {
                hub.Broadcast <- ws.UserMessage{
                    UserID:  userID,
                    Payload: payload,
                }
            }

        case "user.online":
            log.Printf("pulse: user %d is online", p.SenderID)

        default:
            log.Printf("pulse: unhandled type %s", p.Type)
        }
    }
}