package auth

import (
    "database/sql"
    "errors"

    "pulsarmini/internal/models"
    "pulsarmini/internal/db"
    "golang.org/x/crypto/bcrypt"
)

func RegisterUser(database *sql.DB, username, email, password, firstName, lastName string) error {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    _, err = database.Exec(
        db.RebindQuery(`INSERT INTO users (username, email, password_hash, first_name, last_name)
         VALUES (?, ?, ?, ?, ?)`),
        username, email, string(hash), firstName, lastName,
    )
    return err
}

func AuthenticateUser(database *sql.DB, username, password string) (*models.User, error) {
    user := &models.User{}
    row := database.QueryRow(
        db.RebindQuery("SELECT id, username, password_hash FROM users WHERE username = ?"),
        username,
    )
    err := row.Scan(&user.ID, &user.Username, &user.PasswordHash)
    if err == sql.ErrNoRows {
        return nil, errors.New("user not found")
    }
    if err != nil {
        return nil, err
    }
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
        return nil, errors.New("wrong password")
    }
    return user, nil
}

func GetUserByID(database *sql.DB, id int) (*models.User, error) {
    user := &models.User{}
    err := database.QueryRow(
        db.RebindQuery(`SELECT id, username, email, first_name, last_name, created_at
         FROM users WHERE id = ?`), id,
    ).Scan(&user.ID, &user.Username, &user.Email, &user.FirstName, &user.LastName, &user.CreatedAt)
    if err == sql.ErrNoRows {
        return nil, errors.New("user not found")
    }
    return user, err
}

func GetUserByUsername(database *sql.DB, username string) (*models.User, error) {
    user := &models.User{}
    err := database.QueryRow(
        db.RebindQuery(`SELECT id, username, email, first_name, last_name, created_at
         FROM users WHERE username = ?`), username,
    ).Scan(&user.ID, &user.Username, &user.Email, &user.FirstName, &user.LastName, &user.CreatedAt)
    if err == sql.ErrNoRows {
        return nil, errors.New("user not found")
    }
    return user, err
}