package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAuthStore struct {
	db *pgxpool.Pool
}

func NewPostgresAuthStore(db *pgxpool.Pool) *PostgresAuthStore {
	return &PostgresAuthStore{db: db}
}

func (s *PostgresAuthStore) Create(ctx context.Context, username, passwordHash string) (AuthUser, error) {
	const q = `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id, username, password_hash, created_at;
	`

	var u AuthUser
	err := s.db.QueryRow(ctx, q, username, passwordHash).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		// 23505 = unique_violation (username unique constraint)
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return AuthUser{}, ErrUsernameTaken
		}
		return AuthUser{}, err
	}

	return u, nil
}

func (s *PostgresAuthStore) GetByUsername(ctx context.Context, username string) (AuthUser, error) {
	const q = `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = $1;
	`

	var u AuthUser
	err := s.db.QueryRow(ctx, q, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)

	if err != nil {
		return AuthUser{}, ErrUserNotFound
	}

	return u, nil
}
