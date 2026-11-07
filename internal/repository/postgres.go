package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/uzarra/url-shortener/internal/model"
	"github.com/uzarra/url-shortener/migrations"
)

const (
	insertURLQuery         = "INSERT INTO urls (short_url, original_url, user_id) VALUES ($1, $2, NULLIF($3, '')::uuid)"
	insertURLsBatchQuery   = "INSERT INTO urls (short_url, original_url, user_id) SELECT s, o, NULLIF(u, '')::uuid FROM unnest($1::text[], $2::text[], $3::text[]) AS t(s, o, u)"
	deleteURLsBatchQuery   = "UPDATE urls SET is_deleted = TRUE FROM unnest($1::text[], $2::text[]) AS d(short_url, user_id) WHERE urls.short_url = d.short_url AND urls.user_id = d.user_id::uuid AND NOT is_deleted"
	originalURLUniqueIndex = "urls_original_url_uniq"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(db *sql.DB) (*PostgresStorage, error) {
	err := migrateUp(db)
	if err != nil {
		return nil, err
	}
	return &PostgresStorage{
		db: db,
	}, nil
}

func (s *PostgresStorage) Save(ctx context.Context, url model.URL) error {
	_, err := s.db.ExecContext(ctx, insertURLQuery, url.ShortID, url.OriginalURL, url.UserID)
	if err != nil {
		return mapInsertError(err)
	}
	return nil
}

func (s *PostgresStorage) Load(ctx context.Context, id string) (string, error) {
	var originalURL string
	var deletedFlag bool
	err := s.db.QueryRowContext(ctx, "SELECT original_url, is_deleted FROM urls WHERE short_url = $1", id).Scan(&originalURL, &deletedFlag)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select url: %w", err)
	}
	if deletedFlag {
		return "", ErrDeleted
	}
	return originalURL, nil
}

func (s *PostgresStorage) SaveBatch(ctx context.Context, urls []model.URL) error {
	if len(urls) == 0 {
		return nil
	}
	ids := make([]string, len(urls))
	originals := make([]string, len(urls))
	userIDs := make([]string, len(urls))
	for i, u := range urls {
		ids[i] = u.ShortID
		originals[i] = u.OriginalURL
		userIDs[i] = u.UserID
	}
	if _, err := s.db.ExecContext(ctx, insertURLsBatchQuery, ids, originals, userIDs); err != nil {
		return mapInsertError(err)
	}
	return nil
}

func (s *PostgresStorage) GetShortURLByOriginalURL(ctx context.Context, originalURL string) (string, error) {
	var shortURL string
	err := s.db.QueryRowContext(ctx, "SELECT short_url FROM urls WHERE original_url = $1", originalURL).Scan(&shortURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select url: %w", err)
	}
	return shortURL, nil
}

func (s *PostgresStorage) GetURLsByUserID(ctx context.Context, userID string) ([]model.URL, error) {
	if userID == "" {
		return []model.URL{}, nil
	}
	rows, err := s.db.QueryContext(ctx, "SELECT short_url, original_url FROM urls WHERE user_id = $1 AND NOT is_deleted ORDER BY id", userID)
	if err != nil {
		return nil, fmt.Errorf("select urls: %w", err)
	}
	defer rows.Close()
	urls := make([]model.URL, 0)
	for rows.Next() {
		var u model.URL
		if err := rows.Scan(&u.ShortID, &u.OriginalURL); err != nil {
			return nil, fmt.Errorf("row scan: %w", err)
		}
		u.UserID = userID
		urls = append(urls, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return urls, nil
}

func (s *PostgresStorage) DeleteURLs(ctx context.Context, reqs []model.DeleteRequest) error {
	var shortIDs []string
	var userIDs []string
	for _, req := range reqs {
		if req.UserID == "" {
			continue
		}
		shortIDs = append(shortIDs, req.ShortID)
		userIDs = append(userIDs, req.UserID)
	}
	if len(shortIDs) == 0 {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, deleteURLsBatchQuery, shortIDs, userIDs); err != nil {
		return fmt.Errorf("delete urls: %w", err)
	}
	return nil
}

func OpenPostgres(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	return db, nil
}

func migrateUp(db *sql.DB) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("create migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx", driver)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func mapInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		if pgErr.ConstraintName == originalURLUniqueIndex {
			return ErrConflict
		}
		return ErrAlreadyExists
	}
	return fmt.Errorf("insert url: %w", err)
}
