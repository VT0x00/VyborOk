package auth

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

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

// newTestService creates service with real repo and JWTManager
func newTestService(t *testing.T) (*Service, *repository.UserRepository, *sql.DB) {
	t.Helper()
	db := openTestDB(t)
	repo := repository.NewUserRepository(db)
	jwt := NewJWTManager("test-secret", "vyborok-test", time.Minute, time.Hour)
	return NewService(repo, jwt), repo, db
}

// cleanupUser deletes test user by email
func cleanupUser(t *testing.T, db *sql.DB, email string) {
	t.Helper()
	_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE email = $1`, email)
}

func uniqueSuffix() string {
	return uuid.NewString()[:8]
}

// --- Register ---

func TestService_Register_Success(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	suffix := uniqueSuffix()
	email := "reg_ok_" + suffix + "@example.com"
	defer cleanupUser(t, db, email)

	res, err := svc.Register(context.Background(), RegisterInput{
		Email:     email,
		Password:  "secret12345",
		Username:  "user_" + suffix,
		FirstName: "Тест",
		LastName:  "Тестович",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if res.User.ID == uuid.Nil {
		t.Error("expected non-nil user id")
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("expected non-empty tokens")
	}
	if res.User.Email != email {
		t.Errorf("email mismatch: got %q, want %q", res.User.Email, email)
	}
	if res.User.PasswordHash == "secret12345" {
		t.Error("password must be hashed")
	}
}

func TestService_Register_NormalizesEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	suffix := uniqueSuffix()
	emailLower := "reg_norm_" + suffix + "@example.com"
	defer cleanupUser(t, db, emailLower)

	res, err := svc.Register(context.Background(), RegisterInput{
		Email:    "  " + "REG_NORM_" + suffix + "@EXAMPLE.COM  ",
		Password: "secret12345",
		Username: "norm_" + suffix,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if res.User.Email != emailLower {
		t.Errorf("email not normalized: got %q, want %q", res.User.Email, emailLower)
	}
}

func TestService_Register_DuplicateEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	suffix := uniqueSuffix()
	email := "dup_email_" + suffix + "@example.com"
	defer cleanupUser(t, db, email)

	if _, err := svc.Register(context.Background(), RegisterInput{
		Email: email, Password: "secret12345", Username: "u1_" + suffix,
	}); err != nil {
		t.Fatalf("first register: %v", err)
	}

	_, err := svc.Register(context.Background(), RegisterInput{
		Email: email, Password: "other12345", Username: "u2_" + suffix,
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestService_Register_DuplicateUsername(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	suffix := uniqueSuffix()
	username := "dup_user_" + suffix
	email1 := "dup_u1_" + suffix + "@example.com"
	email2 := "dup_u2_" + suffix + "@example.com"
	defer cleanupUser(t, db, email1)
	defer cleanupUser(t, db, email2)

	if _, err := svc.Register(context.Background(), RegisterInput{
		Email: email1, Password: "secret12345", Username: username,
	}); err != nil {
		t.Fatalf("first register: %v", err)
	}

	_, err := svc.Register(context.Background(), RegisterInput{
		Email: email2, Password: "secret12345", Username: username,
	})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestService_Register_InvalidInput(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	cases := []struct {
		name string
		in   RegisterInput
	}{
		{"empty email", RegisterInput{Email: "", Password: "secret12345", Username: "user123"}},
		{"invalid email", RegisterInput{Email: "not-an-email", Password: "secret12345", Username: "user123"}},
		{"short password", RegisterInput{Email: "a@b.c", Password: "123", Username: "user123"}},
		{"long password", RegisterInput{Email: "a@b.c", Password: string(make([]byte, 100)), Username: "user123"}},
		{"empty username", RegisterInput{Email: "a@b.c", Password: "secret12345", Username: ""}},
		{"short username", RegisterInput{Email: "a@b.c", Password: "secret12345", Username: "ab"}},
		{"invalid chars in username", RegisterInput{Email: "a@b.c", Password: "secret12345", Username: "user name!"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Register(context.Background(), tc.in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

// --- Login ---

func TestService_Login_Success(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	suffix := uniqueSuffix()
	email := "login_ok_" + suffix + "@example.com"
	password := "secret12345"
	defer cleanupUser(t, db, email)

	if _, err := svc.Register(context.Background(), RegisterInput{
		Email: email, Password: password, Username: "login_" + suffix,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	res, err := svc.Login(context.Background(), email, password)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.User.Email != email {
		t.Errorf("email mismatch: got %q", res.User.Email)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("expected non-empty tokens")
	}
}

func TestService_Login_NormalizesEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	suffix := uniqueSuffix()
	email := "login_norm_" + suffix + "@example.com"
	password := "secret12345"
	defer cleanupUser(t, db, email)

	if _, err := svc.Register(context.Background(), RegisterInput{
		Email: email, Password: password, Username: "lnorm_" + suffix,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	res, err := svc.Login(context.Background(), "  "+"LOGIN_NORM_"+suffix+"@EXAMPLE.COM  ", password)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.User.Email != email {
		t.Errorf("email mismatch")
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	suffix := uniqueSuffix()
	email := "login_wp_" + suffix + "@example.com"
	defer cleanupUser(t, db, email)

	if _, err := svc.Register(context.Background(), RegisterInput{
		Email: email, Password: "secret12345", Username: "lwp_" + suffix,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := svc.Login(context.Background(), email, "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestService_Login_NonexistentUser(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	_, err := svc.Login(context.Background(), "nobody_"+uniqueSuffix()+"@example.com", "secret12345")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestService_Login_EmptyInput(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	if _, err := svc.Login(context.Background(), "", "secret12345"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for empty email, got %v", err)
	}
	if _, err := svc.Login(context.Background(), "a@b.c", ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for empty password, got %v", err)
	}
}

var _ = models.User{}
