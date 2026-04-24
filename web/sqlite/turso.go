package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/tursodatabase/go-libsql" // remote Turso / libsql driver

	"github.com/gosom/google-maps-scraper/web"
)

// NewTurso opens a libsql remote database (Turso) and returns the same Datastore
// implementation as local SQLite. primaryURL should look like
// libsql://your-db-org.turso.io (scheme required by the driver).
func NewTurso(primaryURL, authToken string) (web.Datastore, error) {
	primaryURL = strings.TrimSpace(primaryURL)
	authToken = strings.TrimSpace(authToken)

	if primaryURL == "" {
		return nil, fmt.Errorf("turso URL is empty")
	}

	if authToken == "" {
		return nil, fmt.Errorf("turso auth token is empty")
	}

	dsn := primaryURL + "?authToken=" + authToken

	db, err := sql.Open("libsql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()

		return nil, err
	}

	if err := createSchema(db); err != nil {
		_ = db.Close()

		return nil, err
	}

	return &repo{db: db}, nil
}
