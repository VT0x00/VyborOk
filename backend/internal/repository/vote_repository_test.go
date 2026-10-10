package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/VT0x00/vyborok/internal/models"
)

func intPtr(i int) *int { return &i }

func newVoteTestPoll(t *testing.T, db *sql.DB, ownerID uuid.UUID) *models.Poll {
	t.Helper()
	ctx := context.Background()
	pollRepo := NewPollRepository(db)

	min, max := 1, 5
	p := &models.Poll{
		UserID:    ownerID,
		Title:     "Vote test " + uuid.NewString()[:8],
		Status:    models.PollStatusActive,
		Anonymous: false,
		Questions: []models.Question{
			{
				Type:     models.QuestionTypeSingle,
				Title:    "Choose one",
				Position: 0,
				Options: []models.Option{
					{Text: "A", Position: 0},
					{Text: "B", Position: 1},
				},
			},
			{
				Type:     models.QuestionTypeScale,
				Title:    "Rate",
				Position: 1,
				ScaleMin: &min,
				ScaleMax: &max,
			},
		},
	}
	if err := pollRepo.Create(ctx, p); err != nil {
		t.Fatalf("create poll: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM polls WHERE id = $1`, p.ID)
	})
	return p
}

func TestVoteRepository_CreateAndGet(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	voteRepo := NewVoteRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newVoteTestPoll(t, db, u.ID)

	v := &models.Vote{
		PollID: p.ID,
		UserID: &u.ID,
		Answers: []models.Answer{
			{QuestionID: p.Questions[0].ID, OptionIDs: []uuid.UUID{p.Questions[0].Options[0].ID}},
			{QuestionID: p.Questions[1].ID, Value: intPtr(3)},
		},
	}
	if err := voteRepo.Create(ctx, v); err != nil {
		t.Fatalf("create vote: %v", err)
	}
	if v.ID == uuid.Nil {
		t.Fatal("expected non-nil vote ID")
	}
	if v.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	got, err := voteRepo.ListByPoll(ctx, p.ID)
	if err != nil {
		t.Fatalf("list by poll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("votes count: got %d, want 1", len(got))
	}
	if got[0].UserID == nil || *got[0].UserID != u.ID {
		t.Error("user_id mismatch")
	}
	if len(got[0].Answers) != 2 {
		t.Fatalf("answers count: got %d, want 2", len(got[0].Answers))
	}
	if got[0].Answers[0].QuestionID != p.Questions[0].ID {
		t.Error("answer 0 question id mismatch")
	}
	if len(got[0].Answers[0].OptionIDs) != 1 || got[0].Answers[0].OptionIDs[0] != p.Questions[0].Options[0].ID {
		t.Error("answer 0 option ids mismatch")
	}
	if got[0].Answers[1].Value == nil || *got[0].Answers[1].Value != 3 {
		t.Error("answer 1 value mismatch")
	}
}

func TestVoteRepository_Create_Anonymous(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	voteRepo := NewVoteRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newVoteTestPoll(t, db, u.ID)

	v := &models.Vote{
		PollID: p.ID,
		UserID: nil,
		Answers: []models.Answer{
			{QuestionID: p.Questions[0].ID, OptionIDs: []uuid.UUID{p.Questions[0].Options[0].ID}},
			{QuestionID: p.Questions[1].ID, Value: intPtr(2)},
		},
	}
	if err := voteRepo.Create(ctx, v); err != nil {
		t.Fatalf("create vote: %v", err)
	}

	got, err := voteRepo.ListByPoll(ctx, p.ID)
	if err != nil {
		t.Fatalf("list by poll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("votes count: got %d, want 1", len(got))
	}
	if got[0].UserID != nil {
		t.Errorf("expected nil user_id, got %v", *got[0].UserID)
	}
}

func TestVoteRepository_Create_UniqueViolation(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	voteRepo := NewVoteRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newVoteTestPoll(t, db, u.ID)

	answers := []models.Answer{
		{QuestionID: p.Questions[0].ID, OptionIDs: []uuid.UUID{p.Questions[0].Options[0].ID}},
		{QuestionID: p.Questions[1].ID, Value: intPtr(3)},
	}
	v1 := &models.Vote{PollID: p.ID, UserID: &u.ID, Answers: answers}
	if err := voteRepo.Create(ctx, v1); err != nil {
		t.Fatalf("create vote 1: %v", err)
	}
	v2 := &models.Vote{PollID: p.ID, UserID: &u.ID, Answers: answers}
	err := voteRepo.Create(ctx, v2)
	if !errors.Is(err, ErrAlreadyVoted) {
		t.Fatalf("expected ErrAlreadyVoted, got %v", err)
	}
}

func TestVoteRepository_Create_MultipleAnonymous(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	voteRepo := NewVoteRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newVoteTestPoll(t, db, u.ID)

	answers := []models.Answer{
		{QuestionID: p.Questions[0].ID, OptionIDs: []uuid.UUID{p.Questions[0].Options[0].ID}},
		{QuestionID: p.Questions[1].ID, Value: intPtr(3)},
	}
	for i := 0; i < 3; i++ {
		v := &models.Vote{PollID: p.ID, UserID: nil, Answers: answers}
		if err := voteRepo.Create(ctx, v); err != nil {
			t.Fatalf("create anonymous vote %d: %v", i, err)
		}
	}

	got, err := voteRepo.ListByPoll(ctx, p.ID)
	if err != nil {
		t.Fatalf("list by poll: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("votes count: got %d, want 3", len(got))
	}
}

func TestVoteRepository_HasVoted(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	voteRepo := NewVoteRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newVoteTestPoll(t, db, u.ID)

	voted, err := voteRepo.HasVoted(ctx, p.ID, u.ID)
	if err != nil {
		t.Fatalf("has voted (before): %v", err)
	}
	if voted {
		t.Error("expected false before voting")
	}

	v := &models.Vote{
		PollID: p.ID,
		UserID: &u.ID,
		Answers: []models.Answer{
			{QuestionID: p.Questions[0].ID, OptionIDs: []uuid.UUID{p.Questions[0].Options[0].ID}},
			{QuestionID: p.Questions[1].ID, Value: intPtr(3)},
		},
	}
	if err := voteRepo.Create(ctx, v); err != nil {
		t.Fatalf("create vote: %v", err)
	}

	voted, err = voteRepo.HasVoted(ctx, p.ID, u.ID)
	if err != nil {
		t.Fatalf("has voted (after): %v", err)
	}
	if !voted {
		t.Error("expected true after voting")
	}
}

func TestVoteRepository_ListByPoll_Order(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	voteRepo := NewVoteRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newVoteTestPoll(t, db, u.ID)

	// Anonymous votes with explicit, ordered created_at values.
	for _, ts := range []string{"2026-01-01 00:00:00", "2026-01-02 00:00:00", "2026-01-03 00:00:00"} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO votes (poll_id, user_id, answers, created_at)
			 VALUES ($1, NULL, '[]'::jsonb, $2)`,
			p.ID, ts,
		)
		if err != nil {
			t.Fatalf("insert vote: %v", err)
		}
	}

	got, err := voteRepo.ListByPoll(ctx, p.ID)
	if err != nil {
		t.Fatalf("list by poll: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("votes count: got %d, want 3", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].CreatedAt.Before(got[i-1].CreatedAt) {
			t.Errorf("votes not ordered by created_at at index %d", i)
		}
	}
}
