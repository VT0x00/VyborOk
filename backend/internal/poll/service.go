package poll

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/VT0x00/vyborok/internal/models"
	"github.com/VT0x00/vyborok/internal/repository"
)

const (
	maxTitleLen       = 255
	maxDescriptionLen = 5000
	maxQuestionTitle  = 500
	maxOptionText     = 500
	maxQuestions      = 50
	maxOptionsPerQ    = 50
	maxScaleRange     = 100
	maxTextLen        = 5000
)

type Service struct {
	polls *repository.PollRepository
	users *repository.UserRepository
}

func NewService(polls *repository.PollRepository, users *repository.UserRepository) *Service {
	return &Service{polls: polls, users: users}
}

type CreateInput struct {
	UserID      uuid.UUID
	Title       string
	Description string
	Anonymous   bool
	Questions   []CreateQuestionInput
}

type CreateQuestionInput struct {
	Type          string
	Title         string
	Description   string
	ScaleMin      int
	ScaleMax      int
	TextMaxLength int
	Options       []string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*models.Poll, error) {
	title := strings.TrimSpace(in.Title)
	if err := validateTextLen(title, 1, maxTitleLen, "title"); err != nil {
		return nil, err
	}
	description := strings.TrimSpace(in.Description)
	if err := validateTextLen(description, 0, maxDescriptionLen, "description"); err != nil {
		return nil, err
	}

	if len(in.Questions) == 0 {
		return nil, fmt.Errorf("%w: at least one question required", ErrInvalidInput)
	}
	if len(in.Questions) > maxQuestions {
		return nil, fmt.Errorf("%w: too many questions (max %d)", ErrInvalidInput, maxQuestions)
	}

	questions := make([]models.Question, 0, len(in.Questions))
	for i, qi := range in.Questions {
		q, err := buildQuestion(qi, i)
		if err != nil {
			return nil, fmt.Errorf("question #%d: %w", i, err)
		}
		questions = append(questions, *q)
	}

	p := &models.Poll{
		UserID:      in.UserID,
		Title:       title,
		Description: description,
		Status:      models.PollStatusActive,
		Anonymous:   in.Anonymous,
		Questions:   questions,
	}
	if err := s.polls.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create poll: %w", err)
	}
	return p, nil
}

// GetByID returns the poll with all questions and options.
// viewerID == uuid.Nil means anonymous caller.
//
// Visibility rules:
//   - owner: full poll, always.
//   - non-owner / anonymous: full poll if active;
//     stripped poll if closed and owner is public;
//     ErrNotFound if closed and owner is private (do not leak existence).
func (s *Service) GetByID(ctx context.Context, id, viewerID uuid.UUID) (*models.Poll, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: poll id is required", ErrInvalidInput)
	}

	poll, err := s.polls.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get poll: %w", err)
	}

	if viewerID == poll.UserID {
		return poll, nil
	}

	owner, err := s.users.GetByID(ctx, poll.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// defensive: cascade delete should have removed the poll too
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get owner: %w", err)
	}

	if poll.Status == models.PollStatusActive {
		return poll, nil
	}

	// closed, non-owner
	if owner.IsPrivate {
		return nil, ErrNotFound
	}

	return stripToStructure(poll), nil
}

// stripToStructure returns a copy of the poll that keeps only the public
// structure (questions and options). For MVP it is identical to the poll
// itself; it is the single place Phase 4 will redact votes and stats from.
func stripToStructure(p *models.Poll) *models.Poll {
	out := *p
	out.Questions = make([]models.Question, len(p.Questions))
	for i := range p.Questions {
		out.Questions[i] = p.Questions[i]
		src := p.Questions[i].Options
		dst := make([]models.Option, len(src))
		copy(dst, src)
		out.Questions[i].Options = dst
	}
	return &out
}

func buildQuestion(in CreateQuestionInput, position int) (*models.Question, error) {
	qType := models.QuestionType(in.Type)
	switch qType {
	case models.QuestionTypeSingle,
		models.QuestionTypeMultiple,
		models.QuestionTypeScale,
		models.QuestionTypeText:
		// ok
	default:
		return nil, fmt.Errorf("%w: unknown question type %q", ErrInvalidInput, in.Type)
	}

	title := strings.TrimSpace(in.Title)
	if err := validateTextLen(title, 1, maxQuestionTitle, "question title"); err != nil {
		return nil, err
	}

	q := &models.Question{
		Type:        qType,
		Title:       title,
		Description: strings.TrimSpace(in.Description),
		Position:    position,
	}

	switch qType {
	case models.QuestionTypeSingle, models.QuestionTypeMultiple:
		if len(in.Options) < 2 {
			return nil, fmt.Errorf("%w: %s requires at least 2 options", ErrInvalidInput, qType)
		}
		if len(in.Options) > maxOptionsPerQ {
			return nil, fmt.Errorf("%w: too many options (max %d)", ErrInvalidInput, maxOptionsPerQ)
		}
		options := make([]models.Option, 0, len(in.Options))
		for oi, raw := range in.Options {
			text := strings.TrimSpace(raw)
			if err := validateTextLen(text, 1, maxOptionText, fmt.Sprintf("option #%d text", oi)); err != nil {
				return nil, err
			}
			options = append(options, models.Option{Text: text, Position: oi})
		}
		q.Options = options

	case models.QuestionTypeScale:
		if in.ScaleMin >= in.ScaleMax {
			return nil, fmt.Errorf("%w: scale_min (%d) must be < scale_max (%d)",
				ErrInvalidInput, in.ScaleMin, in.ScaleMax)
		}
		if in.ScaleMax-in.ScaleMin > maxScaleRange {
			return nil, fmt.Errorf("%w: scale range too large (max %d)", ErrInvalidInput, maxScaleRange)
		}
		minV, maxV := in.ScaleMin, in.ScaleMax
		q.ScaleMin = &minV
		q.ScaleMax = &maxV

	case models.QuestionTypeText:
		if in.TextMaxLength <= 0 {
			return nil, fmt.Errorf("%w: text_max_length must be > 0", ErrInvalidInput)
		}
		if in.TextMaxLength > maxTextLen {
			return nil, fmt.Errorf("%w: text_max_length too large (max %d)", ErrInvalidInput, maxTextLen)
		}
		l := in.TextMaxLength
		q.TextMaxLength = &l
	}

	return q, nil
}

func validateTextLen(s string, minLen, maxLen int, field string) error {
	n := utf8.RuneCountInString(s)
	if n < minLen {
		return fmt.Errorf("%w: %s is required", ErrInvalidInput, field)
	}
	if n > maxLen {
		return fmt.Errorf("%w: %s too long (max %d chars)", ErrInvalidInput, field, maxLen)
	}
	return nil
}
