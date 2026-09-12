package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID  `db:"id"`
	Email         string     `db:"email"`
	PasswordHash  string     `db:"password_hash"`
	Username      string     `db:"username"`
	FirstName     string     `db:"first_name"`
	LastName      string     `db:"last_name"`
	Bio           string     `db:"bio"`
	Links         []string   `db:"links"`         // JSONB в БД
	AvatarURL     string     `db:"avatar_url"`
	IsPrivate     bool       `db:"is_private"`
	PublicFields  []string   `db:"public_fields"` // JSONB в БД
	EmailVerified bool       `db:"email_verified"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}
