package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/VT0x00/vyborok/internal/models"
)

var ErrNotFound = errors.New("not found")

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	const query = `
		INSERT INTO users (email, password_hash, username, first_name, last_name, bio, links, avatar_url, is_private, public_fields, email_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	linksJSON, err := json.Marshal(u.Links)
	if err != nil {
		return fmt.Errorf("marshal links: %w", err)
	}

	fieldsJSON, err := json.Marshal(u.PublicFields)
	if err != nil {
		return fmt.Errorf("marshal public_fields: %w", err)
	}

	err = r.db.QueryRowContext(ctx, query,
		u.Email,
		u.PasswordHash,
		u.Username,
		u.FirstName,
		u.LastName,
		u.Bio,
		linksJSON,
		u.AvatarURL,
		u.IsPrivate,
		fieldsJSON,
		u.EmailVerified,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const query = `
		SELECT id, email, password_hash, username, first_name, last_name, bio, links, avatar_url,
		       is_private, public_fields, email_verified, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	return r.scanOne(ctx, query, id)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	const query = `
		SELECT id, email, password_hash, username, first_name, last_name, bio, links, avatar_url,
		       is_private, public_fields, email_verified, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	return r.scanOne(ctx, query, email)
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	const query = `
		SELECT id, email, password_hash, username, first_name, last_name, bio, links, avatar_url,
		       is_private, public_fields, email_verified, created_at, updated_at
		FROM users
		WHERE username = $1
	`
	return r.scanOne(ctx, query, username)
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exists by email: %w", err)
	}

	return exists, nil
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exists by username: %w", err)
	}

	return exists, nil
}

func (r *UserRepository) Update(ctx context.Context, u *models.User) error {
	const query = `
		UPDATE users
		SET username = $2, first_name = $3, last_name = $4, bio = $5, links = $6,
		    avatar_url = $7, is_private = $8, public_fields = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	linksJSON, err := json.Marshal(u.Links)
	if err != nil {
		return fmt.Errorf("marshal links: %w", err)
	}

	fieldsJSON, err := json.Marshal(u.PublicFields)
	if err != nil {
		return fmt.Errorf("marshal public_fields: %w", err)
	}

	err = r.db.QueryRowContext(ctx, query,
		u.ID, u.Username, u.FirstName, u.LastName, u.Bio, linksJSON,
		u.AvatarURL, u.IsPrivate, fieldsJSON,
	).Scan(&u.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *UserRepository) scanOne(ctx context.Context, query string, args ...any) (*models.User, error) {
	u := &models.User{}
	var linksRaw, fieldsRaw []byte

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Username, &u.FirstName, &u.LastName,
		&u.Bio, &linksRaw, &u.AvatarURL, &u.IsPrivate, &fieldsRaw,
		&u.EmailVerified, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}

	if err := json.Unmarshal(linksRaw, &u.Links); err != nil {
		return nil, fmt.Errorf("unmarshal links: %w", err)
	}
	if err := json.Unmarshal(fieldsRaw, &u.PublicFields); err != nil {
		return nil, fmt.Errorf("unmarshal public_fields: %w", err)
	}

	return u, nil
}
