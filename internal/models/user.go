package models

import "time"

type User struct {
    ID           int
    Username     string
    Email        string
    PasswordHash string
    FirstName    string
    LastName     string
    CreatedAt    time.Time
}

// DisplayName returns the full name if set, otherwise falls back to username
func (u *User) DisplayName() string {
    if u.FirstName != "" || u.LastName != "" {
        return u.FirstName + " " + u.LastName
    }
    return u.Username
}