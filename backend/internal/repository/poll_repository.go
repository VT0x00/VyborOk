package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/VT0x00/vyborok/internal/models"
)

type PollRepository struct {
	db *sql.DB
}

func NewPollRepository(db *sql.DB) *PollRepository {
	return &PollRepository{db: db}
}

const pollColumns = `id, user_id, title, description, status, anonymous, created_at, updated_at`

func (r *PollRepository) Create(ctx context.Context, p *models.Poll) error {
	return withTx(ctx, r.db, func(tx *sql.Tx) error {
		const insertPoll = `
			INSERT INTO polls (user_id, title, description, status, anonymous)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at, updated_at
		`
		err := tx.QueryRowContext(ctx, insertPoll,
			p.UserID, p.Title, p.Description, p.Status, p.Anonymous,
		).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert poll: %w", err)
		}

		const insertQuestion = `
			INSERT INTO questions (poll_id, type, title, description, position, scale_min, scale_max, text_max_length)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, created_at
		`
		const insertOption = `
			INSERT INTO options (question_id, text, position)
			VALUES ($1, $2, $3)
			RETURNING id, created_at
		`

		for qi := range p.Questions {
			q := &p.Questions[qi]
			q.PollID = p.ID
			err := tx.QueryRowContext(ctx, insertQuestion,
				q.PollID, q.Type, q.Title, q.Description, q.Position,
				q.ScaleMin, q.ScaleMax, q.TextMaxLength,
			).Scan(&q.ID, &q.CreatedAt)
			if err != nil {
				return fmt.Errorf("insert question #%d: %w", qi, err)
			}

			for oi := range q.Options {
				o := &q.Options[oi]
				o.QuestionID = q.ID
				err := tx.QueryRowContext(ctx, insertOption,
					o.QuestionID, o.Text, o.Position,
				).Scan(&o.ID, &o.CreatedAt)
				if err != nil {
					return fmt.Errorf("insert option #%d of question #%d: %w", oi, qi, err)
				}
			}
		}
		return nil
	})
}

func (r *PollRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Poll, error) {
	query := `SELECT ` + pollColumns + ` FROM polls WHERE id = $1`
	p := &models.Poll{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.UserID, &p.Title, &p.Description, &p.Status,
		&p.Anonymous, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan poll: %w", err)
	}

	questions, err := r.loadQuestions(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Questions = questions
	return p, nil
}

func (r *PollRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Poll, error) {
	query := `SELECT ` + pollColumns + `
		FROM polls
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query polls: %w", err)
	}
	defer rows.Close()

	polls := []models.Poll{}
	for rows.Next() {
		var p models.Poll
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Title, &p.Description, &p.Status,
			&p.Anonymous, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan poll: %w", err)
		}
		polls = append(polls, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate polls: %w", err)
	}
	return polls, nil
}

func (r *PollRepository) Update(ctx context.Context, p *models.Poll) error {
	const query = `
		UPDATE polls
		SET title = $3, description = $4, anonymous = $5, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		p.ID, p.UserID, p.Title, p.Description, p.Anonymous,
	).Scan(&p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("update poll: %w", err)
	}
	return nil
}

func (r *PollRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM polls WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete poll: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PollRepository) loadQuestions(ctx context.Context, pollID uuid.UUID) ([]models.Question, error) {
	const qQuery = `
		SELECT id, poll_id, type, title, description, position,
		       scale_min, scale_max, text_max_length, created_at
		FROM questions
		WHERE poll_id = $1
		ORDER BY position
	`
	rows, err := r.db.QueryContext(ctx, qQuery, pollID)
	if err != nil {
		return nil, fmt.Errorf("query questions: %w", err)
	}
	defer rows.Close()

	questions := []models.Question{}
	for rows.Next() {
		var q models.Question
		if err := rows.Scan(
			&q.ID, &q.PollID, &q.Type, &q.Title, &q.Description, &q.Position,
			&q.ScaleMin, &q.ScaleMax, &q.TextMaxLength, &q.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan question: %w", err)
		}
		q.Options = []models.Option{}
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate questions: %w", err)
	}

	if len(questions) == 0 {
		return questions, nil
	}

	const oQuery = `
		SELECT o.id, o.question_id, o.text, o.position, o.created_at
		FROM options o
		JOIN questions q ON q.id = o.question_id
		WHERE q.poll_id = $1
		ORDER BY o.question_id, o.position
	`
	oRows, err := r.db.QueryContext(ctx, oQuery, pollID)
	if err != nil {
		return nil, fmt.Errorf("query options: %w", err)
	}
	defer oRows.Close()

	byQuestion := make(map[uuid.UUID][]models.Option)
	for oRows.Next() {
		var o models.Option
		if err := oRows.Scan(&o.ID, &o.QuestionID, &o.Text, &o.Position, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan option: %w", err)
		}
		byQuestion[o.QuestionID] = append(byQuestion[o.QuestionID], o)
	}
	if err := oRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate options: %w", err)
	}

	for i := range questions {
		if opts, ok := byQuestion[questions[i].ID]; ok {
			questions[i].Options = opts
		}
	}
	return questions, nil
}
