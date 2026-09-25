package poll

import (
	"context"
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
}

func NewService(polls *repository.PollRepository) *Service {
	return &Service{polls: polls}
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
