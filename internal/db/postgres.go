package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// Init opens a database connection.
// Uses PostgreSQL when DATABASE_URL env var is set, SQLite otherwise.
func Init() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")

	var driver, source string
	if dsn != "" {
		driver = "postgres"
		source = dsn
		log.Println("db: connecting to PostgreSQL")
	} else {
		driver = "sqlite3"
		source = "./gochat.db"
		log.Println("db: connecting to SQLite (dev)")
	}

	db, err := sql.Open(driver, source)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// RebindQuery converts SQLite-style ? placeholders to PostgreSQL $N style
// when DATABASE_URL is set. Wrap every parameterised query with this.
func RebindQuery(query string) string {
	if os.Getenv("DATABASE_URL") == "" {
		return query
	}
	n := 0
	var b strings.Builder
	for _, ch := range query {
		if ch == '?' {
			n++
			b.WriteString(fmt.Sprintf("$%d", n))
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// IsPostgres returns true when running against PostgreSQL.
func IsPostgres() bool {
	return os.Getenv("DATABASE_URL") != ""
}

func Migrate(database *sql.DB) error {
	var serial, timestamp string
	if IsPostgres() {
		serial    = "SERIAL PRIMARY KEY"
		timestamp = "TIMESTAMPTZ DEFAULT NOW()"
	} else {
		serial    = "INTEGER PRIMARY KEY AUTOINCREMENT"
		timestamp = "DATETIME DEFAULT CURRENT_TIMESTAMP"
	}

	query := `
	CREATE TABLE IF NOT EXISTS users (
		id ` + serial + `,
		username TEXT UNIQUE NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		first_name TEXT NOT NULL DEFAULT '',
		last_name TEXT NOT NULL DEFAULT '',
		created_at ` + timestamp + `
	);

	CREATE TABLE IF NOT EXISTS friend_requests (
		id ` + serial + `,
		sender_id INTEGER NOT NULL,
		receiver_id INTEGER NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at ` + timestamp + `,
		FOREIGN KEY (sender_id) REFERENCES users(id),
		FOREIGN KEY (receiver_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS friends (
		id ` + serial + `,
		user1_id INTEGER NOT NULL,
		user2_id INTEGER NOT NULL,
		created_at ` + timestamp + `,
		FOREIGN KEY (user1_id) REFERENCES users(id),
		FOREIGN KEY (user2_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS conversations (
		id ` + serial + `,
		user1_id INTEGER NOT NULL,
		user2_id INTEGER NOT NULL,
		created_at ` + timestamp + `,
		FOREIGN KEY (user1_id) REFERENCES users(id),
		FOREIGN KEY (user2_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS messages (
		id ` + serial + `,
		conversation_id INTEGER NOT NULL,
		sender_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		is_pulse INTEGER NOT NULL DEFAULT 0,
		created_at ` + timestamp + `,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id),
		FOREIGN KEY (sender_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS pulse_messages (
		id ` + serial + `,
		conversation_id INTEGER NOT NULL,
		sender_id INTEGER NOT NULL,
		receiver_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		delivered INTEGER NOT NULL DEFAULT 0,
		created_at ` + timestamp + `,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id),
		FOREIGN KEY (sender_id) REFERENCES users(id),
		FOREIGN KEY (receiver_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS unread_counts (
		user_id INTEGER NOT NULL,
		conversation_id INTEGER NOT NULL,
		count INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (user_id, conversation_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (conversation_id) REFERENCES conversations(id)
	);
	`

	_, err := database.Exec(query)
	return err
}