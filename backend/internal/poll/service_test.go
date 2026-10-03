package poll

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
	pollRepo := repository.NewPollRepository(db)
	userRepo := repository.NewUserRepository(db)
	return NewService(pollRepo, userRepo), db
}

func uniqueSuffix() string {
	return uuid.NewString()[:8]
}

func newTestUser(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	suffix := uniqueSuffix()

	u := &models.User{
		Email:        "polltest_" + suffix + "@example.com",
		PasswordHash: "$2a$10$dummyhashforvborok",
		Username:     "polluser_" + suffix,
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

func validInput(userID uuid.UUID) CreateInput {
	return CreateInput{
		UserID:      userID,
		Title:       "Любимый язык",
		Description: "Опрос про языки",
		Anonymous:   false,
		Questions: []CreateQuestionInput{
			{
				Type:    "single",
				Title:   "Какой язык чаще?",
				Options: []string{"Go", "Python", "Rust"},
			},
			{
				Type:    "multiple",
				Title:   "Какие языки знаете?",
				Options: []string{"Go", "Python", "Rust", "JS"},
			},
			{
				Type:     "scale",
				Title:    "Насколько довольны Go?",
				ScaleMin: 1,
				ScaleMax: 10,
			},
			{
				Type:          "text",
				Title:         "Что улучшить?",
				TextMaxLength: 500,
			},
		},
	}
}

func TestService_Create_Success(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	userID := newTestUser(t, db)
	p, err := svc.Create(ctx, validInput(userID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if p.ID == uuid.Nil {
		t.Fatal("expected non-nil poll ID")
	}
	if p.UserID != userID {
		t.Errorf("user_id mismatch: got %v, want %v", p.UserID, userID)
	}
	if p.Status != models.PollStatusActive {
		t.Errorf("status must be 'active' on create, got %q", p.Status)
	}
	if p.CreatedAt.IsZero() {
		t.Error("created_at must be set")
	}

	if len(p.Questions) != 4 {
		t.Fatalf("questions count: got %d, want 4", len(p.Questions))
	}

	wantTypes := []models.QuestionType{
		models.QuestionTypeSingle,
		models.QuestionTypeMultiple,
		models.QuestionTypeScale,
		models.QuestionTypeText,
	}
	for i, want := range wantTypes {
		if p.Questions[i].Type != want {
			t.Errorf("q%d type: got %q, want %q", i, p.Questions[i].Type, want)
		}
		if p.Questions[i].Position != i {
			t.Errorf("q%d position: got %d, want %d", i, p.Questions[i].Position, i)
		}
		if p.Questions[i].ID == uuid.Nil {
			t.Errorf("q%d: ID not set", i)
		}
	}

	// single/multiple.
	if len(p.Questions[0].Options) != 3 {
		t.Errorf("q0 options: got %d, want 3", len(p.Questions[0].Options))
	}
	if len(p.Questions[1].Options) != 4 {
		t.Errorf("q1 options: got %d, want 4", len(p.Questions[1].Options))
	}
	for oi, o := range p.Questions[0].Options {
		if o.Position != oi {
			t.Errorf("q0 option %d: position mismatch, got %d", oi, o.Position)
		}
		if o.QuestionID != p.Questions[0].ID {
			t.Errorf("q0 option %d: QuestionID not set", oi)
		}
	}

	// scale.
	if p.Questions[2].ScaleMin == nil || *p.Questions[2].ScaleMin != 1 {
		t.Error("scale_min lost")
	}
	if p.Questions[2].ScaleMax == nil || *p.Questions[2].ScaleMax != 10 {
		t.Error("scale_max lost")
	}
	if p.Questions[2].TextMaxLength != nil {
		t.Error("text_max_length must be nil for scale question")
	}

	// text.
	if p.Questions[3].TextMaxLength == nil || *p.Questions[3].TextMaxLength != 500 {
		t.Error("text_max_length lost")
	}
	if p.Questions[3].ScaleMin != nil || p.Questions[3].ScaleMax != nil {
		t.Error("scale_min/max must be nil for text question")
	}
}

func TestService_Create_TrimsWhitespace(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	userID := newTestUser(t, db)
	in := validInput(userID)
	in.Title = "   Опрос  "
	in.Questions[0].Title = "  Q1  "
	in.Questions[0].Options = []string{"  Go  ", "  Python  ", "  Rust  "}

	p, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Title != "Опрос" {
		t.Errorf("title not trimmed: got %q", p.Title)
	}
	if p.Questions[0].Title != "Q1" {
		t.Errorf("question title not trimmed: got %q", p.Questions[0].Title)
	}
	if p.Questions[0].Options[0].Text != "Go" {
		t.Errorf("option text not trimmed: got %q", p.Questions[0].Options[0].Text)
	}
}

func TestService_Create_NonASCIITitle(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	userID := newTestUser(t, db)
	in := validInput(userID)
	in.Title = strings.Repeat("Я", 255)

	if _, err := svc.Create(ctx, in); err != nil {
		t.Fatalf("create with 255 cyrillic chars: %v", err)
	}
}

func TestService_Create_InvalidInput(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	userID := newTestUser(t, db)

	cases := []struct {
		name   string
		mutate func(*CreateInput)
	}{
		{"empty title", func(in *CreateInput) { in.Title = "" }},
		{"whitespace-only title", func(in *CreateInput) { in.Title = "   " }},
		{"too long title", func(in *CreateInput) {
			in.Title = strings.Repeat("a", maxTitleLen+1)
		}},
		{"too long description", func(in *CreateInput) {
			in.Description = strings.Repeat("a", maxDescriptionLen+1)
		}},
		{"no questions", func(in *CreateInput) { in.Questions = nil }},
		{"too many questions", func(in *CreateInput) {
			in.Questions = make([]CreateQuestionInput, maxQuestions+1)
			for i := range in.Questions {
				in.Questions[i] = CreateQuestionInput{
					Type: "text", Title: "q", TextMaxLength: 100,
				}
			}
		}},
		{"unknown question type", func(in *CreateInput) {
			in.Questions[0].Type = "checkbox"
		}},
		{"empty question title", func(in *CreateInput) {
			in.Questions[0].Title = ""
		}},
		{"too long question title", func(in *CreateInput) {
			in.Questions[0].Title = strings.Repeat("a", maxQuestionTitle+1)
		}},

		// single
		{"single with 1 option", func(in *CreateInput) {
			in.Questions[0].Options = []string{"only"}
		}},
		{"single with 0 options", func(in *CreateInput) {
			in.Questions[0].Options = nil
		}},
		{"single with empty option", func(in *CreateInput) {
			in.Questions[0].Options[1] = "   "
		}},
		{"single with too many options", func(in *CreateInput) {
			opts := make([]string, maxOptionsPerQ+1)
			for i := range opts {
				opts[i] = "o"
			}
			in.Questions[0].Options = opts
		}},

		// scale
		{"scale with min==max", func(in *CreateInput) {
			in.Questions[2].ScaleMin = 5
			in.Questions[2].ScaleMax = 5
		}},
		{"scale with min>max", func(in *CreateInput) {
			in.Questions[2].ScaleMin = 10
			in.Questions[2].ScaleMax = 1
		}},
		{"scale with too large range", func(in *CreateInput) {
			in.Questions[2].ScaleMin = 0
			in.Questions[2].ScaleMax = maxScaleRange + 1
		}},

		// text
		{"text with zero max length", func(in *CreateInput) {
			in.Questions[3].TextMaxLength = 0
		}},
		{"text with negative max length", func(in *CreateInput) {
			in.Questions[3].TextMaxLength = -1
		}},
		{"text with too large max length", func(in *CreateInput) {
			in.Questions[3].TextMaxLength = maxTextLen + 1
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validInput(userID)
			tc.mutate(&in)
			_, err := svc.Create(ctx, in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestService_Create_RollsBackOnRepoError(t *testing.T) {
	_, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	userID := newTestUser(t, db)

	pollRepo := repository.NewPollRepository(db)
	p := &models.Poll{
		UserID: userID,
		Title:  "conflict",
		Status: models.PollStatusActive,
		Questions: []models.Question{
			{Type: models.QuestionTypeText, Title: "Q1", Position: 0, TextMaxLength: intPtr(100)},
			{Type: models.QuestionTypeText, Title: "Q2", Position: 0, TextMaxLength: intPtr(100)}, // dup
		},
	}
	err := pollRepo.Create(ctx, p)
	if err == nil {
		t.Fatal("expected unique violation, got nil")
	}

	var cnt int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM polls WHERE user_id = $1`, userID).Scan(&cnt); err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected 0 polls after rollback, got %d", cnt)
	}
}

// --- GetByID tests (spec 0025) ---

func TestService_GetByID_InvalidID(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()

	// R1: nil UUID is invalid input
	_, err := svc.GetByID(context.Background(), uuid.Nil, uuid.Nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_GetByID_NotFound(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()

	// R6: random UUID → ErrNotFound
	_, err := svc.GetByID(context.Background(), uuid.New(), uuid.Nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_GetByID_OwnerActive(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	ownerID := newTestUser(t, db)
	created, err := svc.Create(ctx, validInput(ownerID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// R1/R2: owner fetches active poll → full data
	got, err := svc.GetByID(ctx, created.ID, ownerID)
	if err != nil {
		t.Fatalf("get by id (owner): %v", err)
	}
	assertFullPoll(t, got, created.ID, ownerID)
}

func TestService_GetByID_ActiveVisibleToAll(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	ownerID := newTestUser(t, db)
	otherID := newTestUser(t, db)
	created, err := svc.Create(ctx, validInput(ownerID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// R3: anonymous fetches active poll → full data
	got, err := svc.GetByID(ctx, created.ID, uuid.Nil)
	if err != nil {
		t.Fatalf("get by id (anonymous): %v", err)
	}
	assertFullPoll(t, got, created.ID, ownerID)

	// R3: other authenticated user fetches active poll → full data
	got, err = svc.GetByID(ctx, created.ID, otherID)
	if err != nil {
		t.Fatalf("get by id (other user): %v", err)
	}
	assertFullPoll(t, got, created.ID, ownerID)
}

func TestService_GetByID_Closed(t *testing.T) {
	svc, db := newTestService(t)
	defer db.Close()
	ctx := context.Background()

	ownerID := newTestUser(t, db)
	otherID := newTestUser(t, db)
	created, err := svc.Create(ctx, validInput(ownerID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// ClosePoll appears in Day 27; flip status directly for now.
	closePoll(t, db, created.ID)

	t.Run("owner gets full poll", func(t *testing.T) {
		// R2: owner fetches closed poll → full data
		got, err := svc.GetByID(ctx, created.ID, ownerID)
		if err != nil {
			t.Fatalf("get by id (owner, closed): %v", err)
		}
		assertFullPoll(t, got, created.ID, ownerID)
		if got.Status != models.PollStatusClosed {
			t.Errorf("status: got %q, want closed", got.Status)
		}
	})

	t.Run("public owner, anonymous gets structure", func(t *testing.T) {
		// R4: anonymous fetches closed poll with public owner → structure present
		got, err := svc.GetByID(ctx, created.ID, uuid.Nil)
		if err != nil {
			t.Fatalf("get by id (anonymous, public owner): %v", err)
		}
		assertFullPoll(t, got, created.ID, ownerID)
	})

	t.Run("public owner, other user gets structure", func(t *testing.T) {
		// R8: other authenticated user, closed poll, public owner → structure present
		got, err := svc.GetByID(ctx, created.ID, otherID)
		if err != nil {
			t.Fatalf("get by id (other user, public owner): %v", err)
		}
		assertFullPoll(t, got, created.ID, ownerID)
	})

	// R7: private owner — poll is not discoverable for non-owners.
	setPrivate(t, db, ownerID)

	t.Run("private owner, anonymous gets ErrNotFound", func(t *testing.T) {
		_, err := svc.GetByID(ctx, created.ID, uuid.Nil)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("private owner, other user gets ErrNotFound", func(t *testing.T) {
		_, err := svc.GetByID(ctx, created.ID, otherID)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("private owner, owner still gets full poll", func(t *testing.T) {
		// R2 exception: owner always sees their poll
		got, err := svc.GetByID(ctx, created.ID, ownerID)
		if err != nil {
			t.Fatalf("get by id (owner, private+closed): %v", err)
		}
		assertFullPoll(t, got, created.ID, ownerID)
	})
}

// --- helpers ---

func intPtr(v int) *int { return &v }

func setPrivate(t *testing.T, db *sql.DB, userID uuid.UUID) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`UPDATE users SET is_private = TRUE WHERE id = $1`, userID); err != nil {
		t.Fatalf("set private: %v", err)
	}
}

func closePoll(t *testing.T, db *sql.DB, pollID uuid.UUID) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`UPDATE polls SET status = 'closed' WHERE id = $1`, pollID); err != nil {
		t.Fatalf("close poll: %v", err)
	}
}

func assertFullPoll(t *testing.T, got *models.Poll, wantID, wantOwner uuid.UUID) {
	t.Helper()
	if got.ID != wantID {
		t.Errorf("id: got %v, want %v", got.ID, wantID)
	}
	if got.UserID != wantOwner {
		t.Errorf("user_id: got %v, want %v", got.UserID, wantOwner)
	}
	if len(got.Questions) != 4 {
		t.Fatalf("questions count: got %d, want 4", len(got.Questions))
	}
	// validInput: single(3 options), multiple(4), scale(0), text(0)
	wantOpts := []int{3, 4, 0, 0}
	for i, q := range got.Questions {
		if len(q.Options) != wantOpts[i] {
			t.Errorf("q%d options: got %d, want %d", i, len(q.Options), wantOpts[i])
		}
	}
}
