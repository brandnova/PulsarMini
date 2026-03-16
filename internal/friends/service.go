package friends

import (
    "database/sql"
    "errors"

    "pulsarmini/internal/models"
    "pulsarmini/internal/db"
)
    
// SendRequest sends a friend request from sender to receiver (looked up by username)
func SendRequest(database *sql.DB, senderID int, receiverUsername string) error {
    // Look up receiver
    var receiverID int
    err := database.QueryRow(
        db.RebindQuery("SELECT id FROM users WHERE username = ?"), 
        receiverUsername,
    ).Scan(&receiverID)
    if err == sql.ErrNoRows {
        return errors.New("user not found")
    }
    if err != nil {
        return err
    }
    if receiverID == senderID {
        return errors.New("you cannot add yourself")
    }

    // Check if already friends
    var exists int
    database.QueryRow(
        db.RebindQuery(`
            SELECT COUNT(*) FROM friends
            WHERE (user1_id = ? AND user2_id = ?) OR (user1_id = ? AND user2_id = ?)`),
        senderID, receiverID, receiverID, senderID,
    ).Scan(&exists)
    if exists > 0 {
        return errors.New("already friends")
    }

    // Check if request already pending
    database.QueryRow(
        db.RebindQuery(`
            SELECT COUNT(*) FROM friend_requests
            WHERE sender_id = ? AND receiver_id = ? AND status = 'pending'`),
        senderID, receiverID,
    ).Scan(&exists)
    if exists > 0 {
        return errors.New("request already sent")
    }

    _, err = database.Exec(
        db.RebindQuery("INSERT INTO friend_requests (sender_id, receiver_id) VALUES (?, ?)"),
        senderID, receiverID,
    )
    return err
}

// AcceptRequest accepts a pending friend request by its ID, ensuring the receiver matches
func AcceptRequest(database *sql.DB, requestID, receiverID int) error {
    var senderID int
    err := database.QueryRow(
        db.RebindQuery(`
            SELECT sender_id FROM friend_requests
            WHERE id = ? AND receiver_id = ? AND status = 'pending'`),
        requestID, receiverID,
    ).Scan(&senderID)
    if err == sql.ErrNoRows {
        return errors.New("request not found")
    }
    if err != nil {
        return err
    }

    tx, err := database.Begin()
    if err != nil {
        return err
    }

    // Update request status
    _, err = tx.Exec(
        db.RebindQuery("UPDATE friend_requests SET status = 'accepted' WHERE id = ?"), 
        requestID,
    )
    if err != nil {
        tx.Rollback()
        return err
    }

    // Create friendship (always store with lower ID first for consistency)
    u1, u2 := senderID, receiverID
    if u2 < u1 {
        u1, u2 = u2, u1
    }
    _, err = tx.Exec(
        db.RebindQuery("INSERT INTO friends (user1_id, user2_id) VALUES (?, ?)"), 
        u1, u2,
    )
    if err != nil {
        tx.Rollback()
        return err
    }

    return tx.Commit()
}

// GetFriends returns all friends for a given user
func GetFriends(database *sql.DB, userID int) ([]models.Friend, error) {
    rows, err := database.Query(
        db.RebindQuery(`
            SELECT u.id, u.username, f.created_at
            FROM friends f
            JOIN users u ON (
                CASE WHEN f.user1_id = ? THEN f.user2_id ELSE f.user1_id END = u.id
            )
            WHERE f.user1_id = ? OR f.user2_id = ?
            ORDER BY u.username ASC`),
        userID, userID, userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var friends []models.Friend
    for rows.Next() {
        var f models.Friend
        if err := rows.Scan(&f.UserID, &f.Username, &f.CreatedAt); err != nil {
            return nil, err
        }
        friends = append(friends, f)
    }
    return friends, nil
}

// GetPendingRequests returns all pending incoming requests for a user
func GetPendingRequests(database *sql.DB, userID int) ([]models.FriendRequest, error) {
    rows, err := database.Query(
        db.RebindQuery(`
            SELECT fr.id, fr.sender_id, u.username, fr.created_at
            FROM friend_requests fr
            JOIN users u ON fr.sender_id = u.id
            WHERE fr.receiver_id = ? AND fr.status = 'pending'
            ORDER BY fr.created_at DESC`),
        userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var requests []models.FriendRequest
    for rows.Next() {
        var r models.FriendRequest
        // Re-using SenderID to carry the username via a temp scan
        var senderUsername string
        if err := rows.Scan(&r.ID, &r.SenderID, &senderUsername, &r.CreatedAt); err != nil {
            return nil, err
        }
        r.Status = senderUsername // borrowing Status field to carry username to handler
        requests = append(requests, r)
    }
    return requests, nil
}

// RejectRequest declines a pending friend request
func RejectRequest(database *sql.DB, requestID, receiverID int) error {
    result, err := database.Exec(
        db.RebindQuery(`
            UPDATE friend_requests SET status = 'rejected'
            WHERE id = ? AND receiver_id = ? AND status = 'pending'`),
        requestID, receiverID,
    )
    if err != nil {
        return err
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return errors.New("request not found")
    }
    return nil
}

// SidebarData holds everything the sidebar partial needs
type SidebarData struct {
    Friends []models.Friend
    Pending []models.FriendRequest
}

func GetSidebarData(database *sql.DB, userID int) (SidebarData, error) {
    friends, err := GetFriends(database, userID)
    if err != nil {
        return SidebarData{}, err
    }
    pending, err := GetPendingRequests(database, userID)
    if err != nil {
        return SidebarData{}, err
    }
    return SidebarData{Friends: friends, Pending: pending}, nil
}