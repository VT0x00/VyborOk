package vote

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

type Service struct {
	votes *repository.VoteRepository
	polls *repository.PollRepository
}

func NewService(votes *repository.VoteRepository, polls *repository.PollRepository) *Service {
	return &Service{votes: votes, polls: polls}
}

// AnswerInput is one answer in a Submit call.
// ValueSet mirrors the proto `value_set` flag (proto3 cannot distinguish
// value=0 from "not set").
type AnswerInput struct {
	QuestionID uuid.UUID
	OptionIDs  []uuid.UUID
	Value      int
	ValueSet   bool
	Text       string
}

type SubmitInput struct {
	PollID  uuid.UUID
	UserID  *uuid.UUID // nil when the caller has no JWT
	Answers []AnswerInput
}

// Submit validates the answers against the poll and stores the vote.
func (s *Service) Submit(ctx context.Context, in SubmitInput) (*models.Vote, error) {
	poll, err := s.polls.GetByID(ctx, in.PollID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get poll: %w", err)
	}

	if poll.Status == models.PollStatusClosed {
		return nil, ErrPollClosed
	}
	if !poll.Anonymous && in.UserID == nil {
		return nil, ErrAuthRequired
	}

	// Identity: recorded whenever a JWT is present, on both anonymous and
	// non-anonymous polls (hybrid semantics, requirements "Design note").
	effectiveUserID := in.UserID

	// Index questions and their valid option ids.
	qByID := make(map[uuid.UUID]models.Question, len(poll.Questions))
	validOpts := make(map[uuid.UUID]map[uuid.UUID]struct{}, len(poll.Questions))
	for _, q := range poll.Questions {
		qByID[q.ID] = q
		opts := make(map[uuid.UUID]struct{}, len(q.Options))
		for _, o := range q.Options {
			opts[o.ID] = struct{}{}
		}
		validOpts[q.ID] = opts
	}

	seen := make(map[uuid.UUID]struct{}, len(in.Answers))
	for _, a := range in.Answers {
		q, ok := qByID[a.QuestionID]
		if !ok {
			return nil, fmt.Errorf("%w: unknown question %s", ErrInvalidInput, a.QuestionID)
		}
		if _, dup := seen[a.QuestionID]; dup {
			return nil, fmt.Errorf("%w: duplicate answer for question %s", ErrInvalidInput, a.QuestionID)
		}
		seen[a.QuestionID] = struct{}{}

		if err := validateAnswer(q, a, validOpts[a.QuestionID]); err != nil {
			return nil, err
		}
	}

	if len(seen) != len(poll.Questions) {
		return nil, fmt.Errorf("%w: answers for %d of %d questions", ErrInvalidInput, len(seen), len(poll.Questions))
	}

	// Repeat check (only when identity is recorded).
	if effectiveUserID != nil {
		voted, err := s.votes.HasVoted(ctx, in.PollID, *effectiveUserID)
		if err != nil {
			return nil, fmt.Errorf("check vote: %w", err)
		}
		if voted {
			return nil, ErrAlreadyVoted
		}
	}

	v := &models.Vote{
		PollID:  in.PollID,
		UserID:  effectiveUserID,
		Answers: make([]models.Answer, 0, len(in.Answers)),
	}
	for _, a := range in.Answers {
		v.Answers = append(v.Answers, toModelAnswer(a))
	}

	if err := s.votes.Create(ctx, v); err != nil {
		if errors.Is(err, repository.ErrAlreadyVoted) {
			return nil, ErrAlreadyVoted
		}
		return nil, fmt.Errorf("create vote: %w", err)
	}
	return v, nil
}

// HasVoted reports whether userID has voted on the poll.
// A nil userID (anonymous caller) always yields false.
func (s *Service) HasVoted(ctx context.Context, pollID uuid.UUID, userID *uuid.UUID) (bool, error) {
	if userID == nil {
		return false, nil
	}
	_, err := s.polls.GetByID(ctx, pollID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("get poll: %w", err)
	}
	voted, err := s.votes.HasVoted(ctx, pollID, *userID)
	if err != nil {
		return false, fmt.Errorf("check vote: %w", err)
	}
	return voted, nil
}

func validateAnswer(q models.Question, a AnswerInput, opts map[uuid.UUID]struct{}) error {
	switch q.Type {
	case models.QuestionTypeSingle:
		if len(a.OptionIDs) != 1 {
			return fmt.Errorf("%w: single question requires exactly 1 option", ErrInvalidInput)
		}
		if _, ok := opts[a.OptionIDs[0]]; !ok {
			return fmt.Errorf("%w: option %s does not belong to the question", ErrInvalidInput, a.OptionIDs[0])
		}
		if a.ValueSet || a.Text != "" {
			return fmt.Errorf("%w: single question expects option_ids only", ErrInvalidInput)
		}
	case models.QuestionTypeMultiple:
		if len(a.OptionIDs) == 0 {
			return fmt.Errorf("%w: multiple question requires at least 1 option", ErrInvalidInput)
		}
		seen := make(map[uuid.UUID]struct{}, len(a.OptionIDs))
		for _, id := range a.OptionIDs {
			if _, ok := opts[id]; !ok {
				return fmt.Errorf("%w: option %s does not belong to the question", ErrInvalidInput, id)
			}
			if _, dup := seen[id]; dup {
				return fmt.Errorf("%w: duplicate option %s", ErrInvalidInput, id)
			}
			seen[id] = struct{}{}
		}
		if a.ValueSet || a.Text != "" {
			return fmt.Errorf("%w: multiple question expects option_ids only", ErrInvalidInput)
		}
	case models.QuestionTypeScale:
		if !a.ValueSet {
			return fmt.Errorf("%w: scale answer requires value_set", ErrInvalidInput)
		}
		if len(a.OptionIDs) != 0 || a.Text != "" {
			return fmt.Errorf("%w: scale question expects value only", ErrInvalidInput)
		}
		if q.ScaleMin == nil || q.ScaleMax == nil {
			return fmt.Errorf("%w: scale question has no range", ErrInvalidInput)
		}
		if a.Value < *q.ScaleMin || a.Value > *q.ScaleMax {
			return fmt.Errorf("%w: scale value %d out of range [%d, %d]", ErrInvalidInput, a.Value, *q.ScaleMin, *q.ScaleMax)
		}
	case models.QuestionTypeText:
		if len(a.OptionIDs) != 0 || a.ValueSet {
			return fmt.Errorf("%w: text question expects text only", ErrInvalidInput)
		}
		t := strings.TrimSpace(a.Text)
		if t == "" {
			return fmt.Errorf("%w: text answer is empty", ErrInvalidInput)
		}
		if q.TextMaxLength == nil {
			return fmt.Errorf("%w: text question has no length limit", ErrInvalidInput)
		}
		if utf8.RuneCountInString(t) > *q.TextMaxLength {
			return fmt.Errorf("%w: text answer too long (max %d chars)", ErrInvalidInput, *q.TextMaxLength)
		}
	}
	return nil
}

func toModelAnswer(a AnswerInput) models.Answer {
	out := models.Answer{
		QuestionID: a.QuestionID,
		OptionIDs:  a.OptionIDs,
		Text:       strings.TrimSpace(a.Text),
	}
	if a.ValueSet {
		v := a.Value
		out.Value = &v
	}
	return out
}
