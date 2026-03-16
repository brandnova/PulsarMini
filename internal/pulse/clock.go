package pulse

import (
    "context"
    "database/sql"
    "log"
    "time"

    "github.com/redis/go-redis/v9"
    ws "pulsarmini/internal/websocket"
    "pulsarmini/internal/db" // Add this import for RebindQuery
)

const PulseInterval = 2 * time.Minute

// nextPulseTime is set when the clock starts and updated after each tick
var nextPulseTime time.Time

// NextPulseIn returns the duration until the next pulse fires
func NextPulseIn() time.Duration {
    d := time.Until(nextPulseTime)
    if d < 0 {
        return 0
    }
    return d
}

func StartClock(ctx context.Context, database *sql.DB, rdb *redis.Client, hub *ws.Hub) {
    nextPulseTime = time.Now().Add(PulseInterval)
    ticker := time.NewTicker(PulseInterval)
    log.Printf("pulse: clock started — firing every %s", PulseInterval)

    go func() {
        for {
            select {
            case t := <-ticker.C:
                log.Printf("pulse: TICK at %s", t.Format("15:04:05"))
                firePulse(ctx, database, rdb, hub)
                nextPulseTime = time.Now().Add(PulseInterval)
            case <-ctx.Done():
                ticker.Stop()
                log.Println("pulse: clock stopped")
                return
            }
        }
    }()
}

func firePulse(ctx context.Context, database *sql.DB, rdb *redis.Client, hub *ws.Hub) {
    rows, err := database.QueryContext(ctx, 
        db.RebindQuery(`
            SELECT pm.id, pm.conversation_id, pm.sender_id, pm.receiver_id,
                   pm.content, u.username
            FROM pulse_messages pm
            JOIN users u ON pm.sender_id = u.id
            WHERE pm.delivered = 0
            ORDER BY pm.created_at ASC
        `),
    )
    if err != nil {
        log.Printf("pulse: failed to fetch pending messages: %v", err)
        return
    }
    defer rows.Close()

    type pendingPulse struct {
        id             int
        conversationID int
        senderID       int
        receiverID     int
        content        string
        senderUsername string
    }

    var pending []pendingPulse
    for rows.Next() {
        var p pendingPulse
        if err := rows.Scan(
            &p.id, &p.conversationID, &p.senderID, &p.receiverID,
            &p.content, &p.senderUsername,
        ); err != nil {
            log.Printf("pulse: scan error: %v", err)
            continue
        }
        pending = append(pending, p)
    }

    if len(pending) == 0 {
        broadcastTick(rdb, ctx)
        return
    }

    log.Printf("pulse: firing %d pulse message(s)", len(pending))

    for _, p := range pending {
        result, err := database.ExecContext(ctx,
            db.RebindQuery("INSERT INTO messages (conversation_id, sender_id, content, is_pulse) VALUES (?, ?, ?, 1)"),
            p.conversationID, p.senderID, p.content,
        )
        if err != nil {
            log.Printf("pulse: failed to save message: %v", err)
            continue
        }

        msgID, _ := result.LastInsertId()

        database.ExecContext(ctx,
            db.RebindQuery("UPDATE pulse_messages SET delivered = 1 WHERE id = ?"), 
            p.id,
        )

        Publish(ctx, rdb, Pulse{
            Type:           "pulse.message.created",
            ConversationID: p.conversationID,
            SenderID:       p.senderID,
            ReceiverID:     p.receiverID,
            SenderUsername: p.senderUsername,
            Content:        p.content,
            MessageID:      int(msgID),
        })
    }

    broadcastTick(rdb, ctx)
}

func broadcastTick(rdb *redis.Client, ctx context.Context) {
    Publish(ctx, rdb, Pulse{
        Type: "pulse.tick",
    })
}