package vote

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/VT0x00/vyborok/internal/models"
	"github.com/VT0x00/vyborok/internal/repository"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set, skipping integration tests")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db := openTestDB(t)
	voteRepo := repository.NewVoteRepository(db)
	pollRepo := repository.NewPollRepository(db)
	return NewService(voteRepo, pollRepo), db
}

func newTestUser(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()[:8]

	u := &models.User{
		Email:        "votetest_" + suffix + "@example.com",
		PasswordHash: "$2a$10$dummyhashforvborok",
		Username:     "voteuser_" + suffix,
		Links:        []string{},
		PublicFields: []string{},
	}
	repo := repository.NewUserRepository(db)
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, u.ID)
	})
	return u.ID
}

func createTestPoll(t *testing.T, db *sql.DB, ownerID uuid.UUID, anonymous bool) *models.Poll {
	t.Helper()
	ctx := context.Background()
	pollRepo := repository.NewPollRepository(db)

	min, max := 1, 10
	maxLen := 20
	p := &models.Poll{
		UserID:      ownerID,
		Title:       "Vote test " + uuid.NewString()[:8],
		Description: "vote service test poll",
		Status:      models.PollStatusActive,
		Anonymous:   anonymous,
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
				Title:    "Rate",
				Position: 2,
				ScaleMin: &min,
				ScaleMax: &max,
			},
			{
				Type:          models.QuestionTypeText,
				Title:         "Comment",
				Position:      3,
				TextMaxLength: &maxLen,
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

func validAnswers(p *models.Poll) []AnswerInput {
	return []AnswerInput{
		{QuestionID: p.Questions[0].ID, OptionIDs: []uuid.UUID{p.Questions[0].Options[0].ID}},
		{QuestionID: p.Questions[1].ID, OptionIDs: []uuid.UUID{p.Questions[1].Options[0].ID, p.Questions[1].Options[1].ID}},
		{QuestionID: p.Questions[2].ID, Value: 5, ValueSet: true},
		{QuestionID: p.Questions[3].ID, Text: "great"},
	}
}

func TestService_Submit_Success(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	p := createTestPoll(t, db, user, false)

	v, err := svc.Submit(ctx, SubmitInput{
		PollID:  p.ID,
		UserID:  &user,
		Answers: validAnswers(p),
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if v.ID == uuid.Nil {
		t.Fatal("expected non-nil vote ID")
	}
	if v.UserID == nil || *v.UserID != user {
		t.Error("expected user_id set")
	}
	if len(v.Answers) != 4 {
		t.Fatalf("answers count: got %d, want 4", len(v.Answers))
	}
}

func TestService_Submit_AnonymousNoJWT(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	owner := newTestUser(t, db)
	p := createTestPoll(t, db, owner, true)

	v, err := svc.Submit(ctx, SubmitInput{
		PollID:  p.ID,
		UserID:  nil,
		Answers: validAnswers(p),
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if v.UserID != nil {
		t.Errorf("expected nil user_id, got %v", *v.UserID)
	}
}

func TestService_Submit_AnonymousWithJWT(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	owner := newTestUser(t, db)
	voter := newTestUser(t, db)
	p := createTestPoll(t, db, owner, true)

	v, err := svc.Submit(ctx, SubmitInput{
		PollID:  p.ID,
		UserID:  &voter,
		Answers: validAnswers(p),
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if v.UserID == nil || *v.UserID != voter {
		t.Error("expected user_id set (hybrid semantics)")
	}
}

func TestService_Submit_Repeat(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	p := createTestPoll(t, db, user, false)

	in := SubmitInput{PollID: p.ID, UserID: &user, Answers: validAnswers(p)}
	if _, err := svc.Submit(ctx, in); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	_, err := svc.Submit(ctx, in)
	if !errors.Is(err, ErrAlreadyVoted) {
		t.Fatalf("expected ErrAlreadyVoted, got %v", err)
	}
}

func TestService_Submit_AnonymousRepeat_NoJWT(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	owner := newTestUser(t, db)
	p := createTestPoll(t, db, owner, true)

	in := SubmitInput{PollID: p.ID, UserID: nil, Answers: validAnswers(p)}
	for i := 0; i < 2; i++ {
		if _, err := svc.Submit(ctx, in); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}
}

func TestService_Submit_AnonymousRepeat_SameJWT(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	owner := newTestUser(t, db)
	voter := newTestUser(t, db)
	p := createTestPoll(t, db, owner, true)

	in := SubmitInput{PollID: p.ID, UserID: &voter, Answers: validAnswers(p)}
	if _, err := svc.Submit(ctx, in); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	_, err := svc.Submit(ctx, in)
	if !errors.Is(err, ErrAlreadyVoted) {
		t.Fatalf("expected ErrAlreadyVoted, got %v", err)
	}
}

func TestService_Submit_PollNotFound(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	_, err := svc.Submit(ctx, SubmitInput{PollID: uuid.New(), UserID: &user})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Submit_PollClosed(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	p := createTestPoll(t, db, user, false)
	if _, err := db.ExecContext(ctx, `UPDATE polls SET status = 'closed' WHERE id = $1`, p.ID); err != nil {
		t.Fatalf("close poll: %v", err)
	}

	_, err := svc.Submit(ctx, SubmitInput{PollID: p.ID, UserID: &user, Answers: validAnswers(p)})
	if !errors.Is(err, ErrPollClosed) {
		t.Fatalf("expected ErrPollClosed, got %v", err)
	}
}

func TestService_Submit_AuthRequired(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	p := createTestPoll(t, db, user, false)

	_, err := svc.Submit(ctx, SubmitInput{PollID: p.ID, UserID: nil, Answers: validAnswers(p)})
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected ErrAuthRequired, got %v", err)
	}
}

// expectInvalidInput creates a fresh poll and voter, applies mutate to the
// valid answers, and expects Submit to fail with ErrInvalidInput.
func expectInvalidInput(t *testing.T, svc *Service, db *sql.DB, mutate func(p *models.Poll, a []AnswerInput) []AnswerInput) {
	t.Helper()
	ctx := context.Background()
	user := newTestUser(t, db)
	p := createTestPoll(t, db, user, false)
	answers := mutate(p, validAnswers(p))
	_, err := svc.Submit(ctx, SubmitInput{PollID: p.ID, UserID: &user, Answers: answers})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Submit_SingleZeroOptions(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[0].OptionIDs = nil
		return a
	})
}

func TestService_Submit_SingleTwoOptions(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[0].OptionIDs = []uuid.UUID{p.Questions[0].Options[0].ID, p.Questions[0].Options[1].ID}
		return a
	})
}

func TestService_Submit_SingleForeignOption(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[0].OptionIDs = []uuid.UUID{p.Questions[1].Options[0].ID}
		return a
	})
}

func TestService_Submit_MultipleZeroOptions(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[1].OptionIDs = nil
		return a
	})
}

func TestService_Submit_MultipleDuplicateOption(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[1].OptionIDs = []uuid.UUID{p.Questions[1].Options[0].ID, p.Questions[1].Options[0].ID}
		return a
	})
}

func TestService_Submit_ScaleOutOfRange(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[2].Value = 11 // max is 10
		return a
	})
}

func TestService_Submit_ScaleBelowRange(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[2].Value = 0 // min is 1
		return a
	})
}

func TestService_Submit_ScaleWithoutValueSet(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[2].ValueSet = false
		return a
	})
}

func TestService_Submit_ScaleWithOptionIDs(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[2].OptionIDs = []uuid.UUID{p.Questions[0].Options[0].ID}
		return a
	})
}

func TestService_Submit_TextTooLong(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[3].Text = strings.Repeat("a", 21) // max is 20
		return a
	})
}

func TestService_Submit_TextEmpty(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[3].Text = "   "
		return a
	})
}

func TestService_Submit_SingleWithValue(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[0].Value = 1
		a[0].ValueSet = true
		return a
	})
}

func TestService_Submit_TextWithValue(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[3].Value = 1
		a[3].ValueSet = true
		return a
	})
}

func TestService_Submit_MissingQuestion(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		return a[:3]
	})
}

func TestService_Submit_DuplicateQuestion(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		return append(a, a[0])
	})
}

func TestService_Submit_ForeignQuestion(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	expectInvalidInput(t, svc, db, func(p *models.Poll, a []AnswerInput) []AnswerInput {
		a[0].QuestionID = uuid.New()
		return a
	})
}

func TestService_Submit_ScaleBoundaries(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	owner := newTestUser(t, db)
	p := createTestPoll(t, db, owner, false)

	for _, val := range []int{1, 10} {
		user := newTestUser(t, db)
		answers := validAnswers(p)
		answers[2].Value = val
		if _, err := svc.Submit(ctx, SubmitInput{PollID: p.ID, UserID: &user, Answers: answers}); err != nil {
			t.Fatalf("submit value %d: %v", val, err)
		}
	}
}

func TestService_HasVoted_PollNotFound(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	_, err := svc.HasVoted(ctx, uuid.New(), &user)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_HasVoted_NoJWT(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	p := createTestPoll(t, db, user, false)

	voted, err := svc.HasVoted(ctx, p.ID, nil)
	if err != nil {
		t.Fatalf("has voted: %v", err)
	}
	if voted {
		t.Error("expected false for anonymous caller")
	}
}

func TestService_HasVoted_NotVoted(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	p := createTestPoll(t, db, user, false)

	voted, err := svc.HasVoted(ctx, p.ID, &user)
	if err != nil {
		t.Fatalf("has voted: %v", err)
	}
	if voted {
		t.Error("expected false before voting")
	}
}

func TestService_HasVoted_Voted(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	user := newTestUser(t, db)
	p := createTestPoll(t, db, user, false)

	if _, err := svc.Submit(ctx, SubmitInput{PollID: p.ID, UserID: &user, Answers: validAnswers(p)}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	voted, err := svc.HasVoted(ctx, p.ID, &user)
	if err != nil {
		t.Fatalf("has voted: %v", err)
	}
	if !voted {
		t.Error("expected true after voting")
	}
}

func TestService_HasVoted_AnonymousPollIdentified(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	owner := newTestUser(t, db)
	voter := newTestUser(t, db)
	p := createTestPoll(t, db, owner, true)

	if _, err := svc.Submit(ctx, SubmitInput{PollID: p.ID, UserID: &voter, Answers: validAnswers(p)}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	voted, err := svc.HasVoted(ctx, p.ID, &voter)
	if err != nil {
		t.Fatalf("has voted: %v", err)
	}
	if !voted {
		t.Error("expected true after voting on an anonymous poll while identified")
	}
}

func TestService_HasVoted_AnonymousVoteNotLinked(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	owner := newTestUser(t, db)
	voter := newTestUser(t, db)
	p := createTestPoll(t, db, owner, true)

	// Vote without a JWT (anonymous).
	if _, err := svc.Submit(ctx, SubmitInput{PollID: p.ID, UserID: nil, Answers: validAnswers(p)}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// The same person now calls HasVoted with a JWT: the anonymous vote
	// is not linked to them.
	voted, err := svc.HasVoted(ctx, p.ID, &voter)
	if err != nil {
		t.Fatalf("has voted: %v", err)
	}
	if voted {
		t.Error("expected false: anonymous vote is not linked to the caller")
	}
}
