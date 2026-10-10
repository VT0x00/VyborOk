package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/VT0x00/vyborok/internal/models"
)

// ErrAlreadyVoted is returned when a (poll_id, user_id) unique constraint
// is violated on insert.
var ErrAlreadyVoted = errors.New("already voted")

type VoteRepository struct {
	db *sql.DB
}

func NewVoteRepository(db *sql.DB) *VoteRepository {
	return &VoteRepository{db: db}
}

// Create inserts a vote. On a (poll_id, user_id) unique violation it
// returns ErrAlreadyVoted.
func (r *VoteRepository) Create(ctx context.Context, v *models.Vote) error {
	answers, err := json.Marshal(v.Answers)
	if err != nil {
		return fmt.Errorf("marshal answers: %w", err)
	}

	const query = `
		INSERT INTO votes (poll_id, user_id, answers)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	err = r.db.QueryRowContext(ctx, query, v.PollID, v.UserID, answers).
		Scan(&v.ID, &v.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyVoted
		}
		return fmt.Errorf("insert vote: %w", err)
	}
	return nil
}

// HasVoted reports whether userID has a vote row for the poll.
func (r *VoteRepository) HasVoted(ctx context.Context, pollID, userID uuid.UUID) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM votes WHERE poll_id = $1 AND user_id = $2 LIMIT 1`,
		pollID, userID,
	).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check vote: %w", err)
	}
	return true, nil
}

// ListByPoll returns every vote for the poll, oldest first.
func (r *VoteRepository) ListByPoll(ctx context.Context, pollID uuid.UUID) ([]models.Vote, error) {
	const query = `
		SELECT id, poll_id, user_id, answers, created_at
		FROM votes
		WHERE poll_id = $1
		ORDER BY created_at
	`
	rows, err := r.db.QueryContext(ctx, query, pollID)
	if err != nil {
		return nil, fmt.Errorf("query votes: %w", err)
	}
	defer rows.Close()

	votes := []models.Vote{}
	for rows.Next() {
		var (
			v         models.Vote
			userID    *uuid.UUID
			answers   []byte
			createdAt time.Time
		)
		if err := rows.Scan(&v.ID, &v.PollID, &userID, &answers, &createdAt); err != nil {
			return nil, fmt.Errorf("scan vote: %w", err)
		}
		v.UserID = userID
		v.CreatedAt = createdAt
		if err := json.Unmarshal(answers, &v.Answers); err != nil {
			return nil, fmt.Errorf("unmarshal answers: %w", err)
		}
		votes = append(votes, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate votes: %w", err)
	}
	return votes, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation
// (23505).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}
