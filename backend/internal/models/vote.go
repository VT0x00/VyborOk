package models

import (
	"time"

	"github.com/google/uuid"
)

// Vote is a single submission of answers to a poll.
// UserID is nil for anonymous votes (no JWT at submission time).
type Vote struct {
	ID        uuid.UUID
	PollID    uuid.UUID
	UserID    *uuid.UUID
	Answers   []Answer
	CreatedAt time.Time
}

// Answer is the stored answer to one question.
// Exactly one of OptionIDs / Value / Text is populated, matching the
// question type (single/multiple → OptionIDs, scale → Value, text → Text).
type Answer struct {
	QuestionID uuid.UUID   `json:"question_id"`
	OptionIDs  []uuid.UUID `json:"option_ids,omitempty"`
	Value      *int        `json:"value,omitempty"`
	Text       string      `json:"text,omitempty"`
}
