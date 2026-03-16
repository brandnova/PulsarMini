package chat

import (
    "database/sql"

    "pulsarmini/internal/models"
    "pulsarmini/internal/db"
)

// GetOrCreateConversation finds an existing conversation between two users
// or creates one if it doesn't exist yet
func GetOrCreateConversation(database *sql.DB, userID, friendID int) (int, error) {
    var convID int
    err := database.QueryRow(
        db.RebindQuery(`
            SELECT id FROM conversations
            WHERE (user1_id = ? AND user2_id = ?)
               OR (user1_id = ? AND user2_id = ?)`),
        userID, friendID, friendID, userID,
    ).Scan(&convID)

    if err == sql.ErrNoRows {
        // No conversation yet — create one
        u1, u2 := userID, friendID
        if u2 < u1 {
            u1, u2 = u2, u1
        }
        result, err := database.Exec(
            db.RebindQuery("INSERT INTO conversations (user1_id, user2_id) VALUES (?, ?)"), 
            u1, u2,
        )
        if err != nil {
            return 0, err
        }
        id, err := result.LastInsertId()
        return int(id), err
    }

    return convID, err
}

// SaveMessage persists a message to the database
func SaveMessage(database *sql.DB, conversationID, senderID int, content string) (models.Message, error) {
    result, err := database.Exec(
        db.RebindQuery("INSERT INTO messages (conversation_id, sender_id, content) VALUES (?, ?, ?)"),
        conversationID, senderID, content,
    )
    if err != nil {
        return models.Message{}, err
    }

    id, _ := result.LastInsertId()

    // Fetch back with sender username for immediate rendering
    var msg models.Message
    err = database.QueryRow(
        db.RebindQuery(`
            SELECT m.id, m.conversation_id, m.sender_id, u.username, m.content, m.created_at
            FROM messages m
            JOIN users u ON m.sender_id = u.id
            WHERE m.id = ?`), 
        id,
    ).Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.SenderUsername, &msg.Content, &msg.CreatedAt)

    return msg, err
}

// GetMessages returns all messages for a conversation, oldest first
func GetMessages(database *sql.DB, conversationID int) ([]models.Message, error) {
    rows, err := database.Query(
        db.RebindQuery(`
            SELECT m.id, m.conversation_id, m.sender_id, u.username, m.content, m.created_at
            FROM messages m
            JOIN users u ON m.sender_id = u.id
            WHERE m.conversation_id = ?
            ORDER BY m.created_at ASC`),
        conversationID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []models.Message
    for rows.Next() {
        var msg models.Message
        if err := rows.Scan(
            &msg.ID, &msg.ConversationID, &msg.SenderID,
            &msg.SenderUsername, &msg.Content, &msg.CreatedAt,
        ); err != nil {
            return nil, err
        }
        messages = append(messages, msg)
    }
    return messages, nil
}

// AreFriends checks that two users are actually friends before allowing chat
func AreFriends(database *sql.DB, userID, friendID int) (bool, error) {
    var count int
    err := database.QueryRow(
        db.RebindQuery(`
            SELECT COUNT(*) FROM friends
            WHERE (user1_id = ? AND user2_id = ?)
               OR (user1_id = ? AND user2_id = ?)`),
        userID, friendID, friendID, userID,
    ).Scan(&count)
    return count > 0, err
}

// IncrementUnread bumps the unread counter for a user in a conversation
func IncrementUnread(database *sql.DB, userID, conversationID int) error {
    _, err := database.Exec(
        db.RebindQuery(`
            INSERT INTO unread_counts (user_id, conversation_id, count)
            VALUES (?, ?, 1)
            ON CONFLICT(user_id, conversation_id) DO UPDATE SET count = count + 1`),
        userID, conversationID,
    )
    return err
}

// ClearUnread resets the unread counter when a user opens a conversation
func ClearUnread(database *sql.DB, userID, conversationID int) error {
    _, err := database.Exec(
        db.RebindQuery("DELETE FROM unread_counts WHERE user_id=? AND conversation_id=?"),
        userID, conversationID,
    )
    return err
}

// GetUnreadCounts returns a map of conversationID → unread count for a user
func GetUnreadCounts(database *sql.DB, userID int) (map[int]int, error) {
    rows, err := database.Query(
        db.RebindQuery("SELECT conversation_id, count FROM unread_counts WHERE user_id=?"), 
        userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    m := make(map[int]int)
    for rows.Next() {
        var cid, count int
        if err := rows.Scan(&cid, &count); err != nil {
            return nil, err
        }
        m[cid] = count
    }
    return m, nil
}