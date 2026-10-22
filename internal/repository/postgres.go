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
	insertURLQuery         = "INSERT INTO urls (short_url, original_url) VALUES ($1, $2)"
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

func (s *PostgresStorage) Save(ctx context.Context, id string, originalURL string) error {
	_, err := s.db.ExecContext(ctx, insertURLQuery, id, originalURL)
	if err != nil {
		isUniqueViolationErr, constraintName := isUniqueViolation(err)
		if isUniqueViolationErr {
			if constraintName == originalURLUniqueIndex {
				return ErrConflict
			}
			return ErrAlreadyExists
		}
		return fmt.Errorf("insert url: %w", err)
	}
	return nil
}

func (s *PostgresStorage) Load(ctx context.Context, id string) (string, error) {
	var originalURL string
	err := s.db.QueryRowContext(ctx, "SELECT original_url FROM urls WHERE short_url = $1", id).Scan(&originalURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select url: %w", err)
	}
	return originalURL, nil
}

func (s *PostgresStorage) SaveBatch(ctx context.Context, urls []model.URL) error {
	if len(urls) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, insertURLQuery)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()
	for _, u := range urls {
		if _, err := stmt.ExecContext(ctx, u.ShortID, u.OriginalURL); err != nil {
			isUniqueViolationErr, constraintName := isUniqueViolation(err)
			if isUniqueViolationErr {
				if constraintName == originalURLUniqueIndex {
					return ErrConflict
				}
				return ErrAlreadyExists
			}
			return fmt.Errorf("insert url: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
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

func isUniqueViolation(err error) (bool, string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgerrcode.UniqueViolation, pgErr.ConstraintName
	}
	return false, ""
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
