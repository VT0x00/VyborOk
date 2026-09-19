package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/VT0x00/vyborok/internal/models"
)

func newPoll(userID uuid.UUID) *models.Poll {
	min, max := 1, 5
	return &models.Poll{
		UserID:      userID,
		Title:       "Poll " + uuid.NewString()[:8],
		Description: "integration test poll",
		Status:      models.PollStatusActive,
		Anonymous:   false,
		Questions: []models.Question{
			{
				Type:     models.QuestionTypeSingle,
				Title:    "Choose one",
				Position: 0,
				Options: []models.Option{
					{Text: "A", Position: 0},
					{Text: "B", Position: 1},
					{Text: "C", Position: 2},
				},
			},
			{
				Type:     models.QuestionTypeMultiple,
				Title:    "Choose many",
				Position: 1,
				Options: []models.Option{
					{Text: "X", Position: 0},
					{Text: "Y", Position: 1},
				},
			},
			{
				Type:     models.QuestionTypeScale,
				Title:    "Rate us",
				Position: 2,
				ScaleMin: &min,
				ScaleMax: &max,
			},
		},
	}
}

func TestPollRepository_CreateAndGetByID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	pollRepo := NewPollRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newPoll(u.ID)
	if err := pollRepo.Create(ctx, p); err != nil {
		t.Fatalf("create poll: %v", err)
	}

	if p.ID == uuid.Nil {
		t.Fatal("expected non-nil poll ID")
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
	for qi := range p.Questions {
		if p.Questions[qi].ID == uuid.Nil {
			t.Errorf("question #%d: expected non-nil ID", qi)
		}
		if p.Questions[qi].PollID != p.ID {
			t.Errorf("question #%d: PollID not set", qi)
		}
		for oi := range p.Questions[qi].Options {
			if p.Questions[qi].Options[oi].ID == uuid.Nil {
				t.Errorf("q#%d/o#%d: expected non-nil ID", qi, oi)
			}
			if p.Questions[qi].Options[oi].QuestionID != p.Questions[qi].ID {
				t.Errorf("q#%d/o#%d: QuestionID not set", qi, oi)
			}
		}
	}

	got, err := pollRepo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Title != p.Title {
		t.Errorf("title mismatch: got %q, want %q", got.Title, p.Title)
	}
	if got.UserID != u.ID {
		t.Errorf("user_id mismatch: got %v, want %v", got.UserID, u.ID)
	}
	if got.Status != models.PollStatusActive {
		t.Errorf("status mismatch: got %q", got.Status)
	}
	if len(got.Questions) != 3 {
		t.Fatalf("questions count: got %d, want 3", len(got.Questions))
	}
	for i, q := range got.Questions {
		if q.Position != i {
			t.Errorf("question #%d position: got %d, want %d", i, q.Position, i)
		}
	}
	if len(got.Questions[0].Options) != 3 {
		t.Errorf("q0 options: got %d, want 3", len(got.Questions[0].Options))
	}
	if len(got.Questions[1].Options) != 2 {
		t.Errorf("q1 options: got %d, want 2", len(got.Questions[1].Options))
	}
	if got.Questions[2].Options == nil {
		t.Error("q2 (scale): Options must be empty slice, not nil")
	}
	if len(got.Questions[2].Options) != 0 {
		t.Errorf("q2 (scale): got %d options, want 0", len(got.Questions[2].Options))
	}
	if got.Questions[2].ScaleMin == nil || *got.Questions[2].ScaleMin != 1 {
		t.Error("scale_min lost")
	}
	if got.Questions[2].ScaleMax == nil || *got.Questions[2].ScaleMax != 5 {
		t.Error("scale_max lost")
	}
}

func TestPollRepository_Create_RollsBackOnError(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	pollRepo := NewPollRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newPoll(u.ID)
	p.Questions[1].Position = 0

	err := pollRepo.Create(ctx, p)
	if err == nil {
		t.Fatal("expected unique violation, got nil")
	}

	var cnt int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM polls WHERE user_id = $1`, u.ID).Scan(&cnt); err != nil {
		t.Fatalf("count polls: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected 0 polls after rollback, got %d", cnt)
	}
}

func TestPollRepository_GetByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewPollRepository(db)

	_, err := repo.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPollRepository_ListByUser(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	pollRepo := NewPollRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	for i := 0; i < 3; i++ {
		p := newPoll(u.ID)
		p.Title = fmt.Sprintf("poll-%d", i)
		if err := pollRepo.Create(ctx, p); err != nil {
			t.Fatalf("create poll %d: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	all, err := pollRepo.ListByUser(ctx, u.ID, 10, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 polls, got %d", len(all))
	}
	// DESC: свежий (poll-2) первым
	if all[0].Title != "poll-2" {
		t.Errorf("expected newest first, got %q", all[0].Title)
	}
	if all[2].Title != "poll-0" {
		t.Errorf("expected oldest last, got %q", all[2].Title)
	}

	page1, err := pollRepo.ListByUser(ctx, u.ID, 2, 0)
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("page1: got %d, want 2", len(page1))
	}
	page2, err := pollRepo.ListByUser(ctx, u.ID, 2, 2)
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if len(page2) != 1 {
		t.Errorf("page2: got %d, want 1", len(page2))
	}

	other := newUser()
	if err := userRepo.Create(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}
	defer cleanup(t, db, other.ID)

	otherPolls, err := pollRepo.ListByUser(ctx, other.ID, 10, 0)
	if err != nil {
		t.Fatalf("list other: %v", err)
	}
	if len(otherPolls) != 0 {
		t.Errorf("expected 0 polls for other user, got %d", len(otherPolls))
	}
}

func TestPollRepository_Update(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	pollRepo := NewPollRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newPoll(u.ID)
	if err := pollRepo.Create(ctx, p); err != nil {
		t.Fatalf("create poll: %v", err)
	}

	p.Title = "Updated title"
	p.Description = "Updated description"
	p.Anonymous = true

	if err := pollRepo.Update(ctx, p); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := pollRepo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Title != "Updated title" {
		t.Errorf("title mismatch: got %q", got.Title)
	}
	if got.Description != "Updated description" {
		t.Errorf("description mismatch: got %q", got.Description)
	}
	if !got.Anonymous {
		t.Error("anonymous should be true")
	}
	if len(got.Questions) != 3 {
		t.Errorf("questions should be intact, got %d", len(got.Questions))
	}
}

func TestPollRepository_Update_WrongOwner(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	pollRepo := NewPollRepository(db)

	owner := newUser()
	if err := userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer cleanup(t, db, owner.ID)

	other := newUser()
	if err := userRepo.Create(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}
	defer cleanup(t, db, other.ID)

	p := newPoll(owner.ID)
	if err := pollRepo.Create(ctx, p); err != nil {
		t.Fatalf("create poll: %v", err)
	}

	p.UserID = other.ID
	p.Title = "hacked"

	err := pollRepo.Update(ctx, p)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound when wrong owner, got %v", err)
	}

	got, err := pollRepo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title == "hacked" {
		t.Error("poll was updated by wrong owner!")
	}
}

func TestPollRepository_Delete(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	pollRepo := NewPollRepository(db)

	u := newUser()
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer cleanup(t, db, u.ID)

	p := newPoll(u.ID)
	if err := pollRepo.Create(ctx, p); err != nil {
		t.Fatalf("create poll: %v", err)
	}
	questionIDs := make([]uuid.UUID, 0, len(p.Questions))
	for _, q := range p.Questions {
		questionIDs = append(questionIDs, q.ID)
	}

	if err := pollRepo.Delete(ctx, p.ID, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := pollRepo.GetByID(ctx, p.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	var qCnt int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM questions WHERE id = ANY($1)`, questionIDs).Scan(&qCnt); err != nil {
		t.Fatalf("count questions: %v", err)
	}
	if qCnt != 0 {
		t.Errorf("expected 0 questions after cascade, got %d", qCnt)
	}
}

func TestPollRepository_Delete_WrongOwner(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userRepo := NewUserRepository(db)
	pollRepo := NewPollRepository(db)

	owner := newUser()
	if err := userRepo.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer cleanup(t, db, owner.ID)

	other := newUser()
	if err := userRepo.Create(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}
	defer cleanup(t, db, other.ID)

	p := newPoll(owner.ID)
	if err := pollRepo.Create(ctx, p); err != nil {
		t.Fatalf("create poll: %v", err)
	}

	err := pollRepo.Delete(ctx, p.ID, other.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound when wrong owner, got %v", err)
	}

	if _, err := pollRepo.GetByID(ctx, p.ID); err != nil {
		t.Errorf("poll should still exist, got err: %v", err)
	}
}
