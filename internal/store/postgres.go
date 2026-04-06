package store

import (
	"database/sql"
	"fmt"
	"time"

	"url-shortening-service/internal/shortener"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	errNotFound = fmt.Errorf("item not found")
)

type URL struct {
	ID          int64     `db:"id" json:"id"`
	ShortCode   string    `db:"short_code" json:"shortCode"`
	OriginalURL string    `db:"original_url" json:"url"`
	AccessCount int64     `db:"access_count" json:"accessCount"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

type Store struct {
	db *sqlx.DB
}

func New(connStr string) (*Store, error) {
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS urls (
			id BIGSERIAL PRIMARY KEY,
			short_code VARCHAR(10) UNIQUE NOT NULL,
			original_url TEXT NOT NULL,
			access_count BIGINT DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}

func (s *Store) Create(originalUrl string) (*URL, error) {
	shortCode := shortener.Generate()

	var url URL
	err := s.db.QueryRowx(`
		INSERT INTO urls (short_code, original_url) 
		VALUES ($1, $2) 
		RETURNING id, short_code, original_url, access_count, created_at, expires_at
	`, shortCode, originalUrl).StructScan(&url)
	if err != nil {
		return nil, err
	}

	return &url, nil
}

func (s *Store) GetByShortCode(code string) (*URL, error) {
	var url URL
	err := s.db.Get(&url, `SELECT * FROM urls WHERE short_code = $1`, code)
	if err != nil {
		return nil, err
	}

	return &url, nil
}

func (s *Store) Update(code string, newUrl string) (*URL, error) {
	var url URL
	err := s.db.QueryRowx(`
		UPDATE urls
		SET original_url = $1, updated_at = NOW()
		WHERE short_code = $3
		RETURNING id, short_code, original_url, access_count, created_at, updated_at
	`, newUrl, code).StructScan(&url)
	if err == sql.ErrNoRows {
		return nil, err
	}

	return &url, err
}

func (s *Store) Delete(code string) error {
	result, err := s.db.Exec(`
		DELETE FROM urls
		WHERE short_code = $1
	`, code)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errNotFound
	}

	return nil
}

func (s *Store) IncrementAccessCount(code string) error {
	_, err := s.db.Exec(`
		UPDATE urls
		SET access_count = access_count + 1
		WHERE short_code = $1
	`, code)

	return err
}

func ItemNotFound(err error) bool {
	return err == errNotFound
}
