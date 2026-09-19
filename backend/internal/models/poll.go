package models

import (
	"time"

	"github.com/google/uuid"
)

type PollStatus string

const (
	PollStatusActive PollStatus = "active"
	PollStatusClosed PollStatus = "closed"
)

// single   — один вариант из options
// multiple — несколько вариантов из options
// scale    — число в диапазоне [scale_min, scale_max]
// text     — свободный текст длиной до text_max_length
type QuestionType string

const (
	QuestionTypeSingle   QuestionType = "single"
	QuestionTypeMultiple QuestionType = "multiple"
	QuestionTypeScale    QuestionType = "scale"
	QuestionTypeText     QuestionType = "text"
)

type Poll struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Title       string
	Description string
	Status      PollStatus
	Anonymous   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Questions []Question
}

type Question struct {
	ID            uuid.UUID
	PollID        uuid.UUID
	Type          QuestionType
	Title         string
	Description   string
	Position      int
	ScaleMin      *int
	ScaleMax      *int
	TextMaxLength *int
	CreatedAt     time.Time

	Options []Option
}

type Option struct {
	ID         uuid.UUID
	QuestionID uuid.UUID
	Text       string
	Position   int
	CreatedAt  time.Time
}
