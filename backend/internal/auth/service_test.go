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

func registerForTest(t *testing.T, svc *Service, db *sql.DB, prefix string) (*AuthResult, string) {
	t.Helper()
	suffix := uniqueSuffix()
	email := prefix + "_" + suffix + "@example.com"
	res, err := svc.Register(context.Background(), RegisterInput{
		Email:    email,
		Password: "secret12345",
		Username: prefix + "_" + suffix,
	})
	if err != nil {
		t.Fatalf("register %s: %v", prefix, err)
	}
	t.Cleanup(func() { cleanupUser(t, db, email) })
	return res, email
}

// --- Refresh ---

func TestService_Refresh_Success(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "refresh_ok")

	res, err := svc.Refresh(context.Background(), reg.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if res.User.ID != reg.User.ID {
		t.Errorf("user id mismatch: got %v, want %v", res.User.ID, reg.User.ID)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("expected non-empty tokens")
	}
}

func TestService_Refresh_AccessTokenRejected(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "refresh_access")

	_, err := svc.Refresh(context.Background(), reg.AccessToken)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken when access token is used as refresh, got %v", err)
	}
}

func TestService_Refresh_InvalidToken(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	_, err := svc.Refresh(context.Background(), "not-a-valid-token")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestService_Refresh_EmptyToken(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	_, err := svc.Refresh(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_Refresh_UserDeleted(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	suffix := uniqueSuffix()
	email := "refresh_del_" + suffix + "@example.com"

	reg, err := svc.Register(context.Background(), RegisterInput{
		Email: email, Password: "secret12345", Username: "rdel_" + suffix,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	cleanupUser(t, db, email)

	_, err = svc.Refresh(context.Background(), reg.RefreshToken)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for deleted user, got %v", err)
	}
}

// --- GetMe ---

func TestService_GetMe_Success(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "getme_ok")

	got, err := svc.GetMe(context.Background(), reg.User.ID)
	if err != nil {
		t.Fatalf("get me: %v", err)
	}
	if got.ID != reg.User.ID {
		t.Errorf("id mismatch: got %v, want %v", got.ID, reg.User.ID)
	}
	if got.Email != reg.User.Email {
		t.Errorf("email mismatch: got %q, want %q", got.Email, reg.User.Email)
	}
}

func TestService_GetMe_NotFound(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	_, err := svc.GetMe(context.Background(), uuid.New())
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

// --- UpdateProfile ---

func TestService_UpdateProfile_UpdatesBio(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "up_bio")

	updated, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		Bio: "new bio",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Bio != "new bio" {
		t.Errorf("bio mismatch in returned user: got %q", updated.Bio)
	}

	got, err := svc.GetMe(context.Background(), reg.User.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Bio != "new bio" {
		t.Errorf("bio not persisted: got %q", got.Bio)
	}
}

// Ключевой тест: is_private=true в запросе БЕЗ is_private_set не должен
// трогать значение в БД. Это суть пары field + field_set в proto3.
func TestService_UpdateProfile_IsPrivateSetFalse_DoesNotChange(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "up_ip_keep")

	// Включаем приватность (is_private_set=true).
	_, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		IsPrivate:    true,
		IsPrivateSet: true,
	})
	if err != nil {
		t.Fatalf("set private: %v", err)
	}

	// Присылаем is_private=false БЕЗ is_private_set — не должно ничего изменить.
	updated, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		IsPrivate:    false,
		IsPrivateSet: false,
		Bio:          "только био",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.IsPrivate {
		t.Error("is_private must NOT change without is_private_set=true")
	}
	if updated.Bio != "только био" {
		t.Errorf("bio should have been updated, got %q", updated.Bio)
	}
}

func TestService_UpdateProfile_IsPrivateSetTrue_SetsFalse(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "up_ip_false")

	// Сначала делаем приватным.
	_, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		IsPrivate:    true,
		IsPrivateSet: true,
	})
	if err != nil {
		t.Fatalf("set private: %v", err)
	}

	// Теперь с is_private_set=true и is_private=false — должно выключиться.
	updated, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		IsPrivate:    false,
		IsPrivateSet: true,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.IsPrivate {
		t.Error("is_private should have been set to false")
	}
}

func TestService_UpdateProfile_ChangesUsername(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "up_uname")

	newUsername := "changed_" + uniqueSuffix()
	updated, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		Username: newUsername,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Username != newUsername {
		t.Errorf("username mismatch: got %q, want %q", updated.Username, newUsername)
	}
}

func TestService_UpdateProfile_UsernameTaken(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	userA, _ := registerForTest(t, svc, db, "up_taken_a")
	userB, _ := registerForTest(t, svc, db, "up_taken_b")

	_, err := svc.UpdateProfile(context.Background(), userB.User.ID, UpdateProfileInput{
		Username: userA.User.Username,
	})
	if !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestService_UpdateProfile_InvalidUsername(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "up_inv_uname")

	_, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		Username: "ab",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestService_UpdateProfile_PublicFieldsReplaced(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "up_pf")

	_, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		PublicFields:    []string{"bio", "first_name"},
		PublicFieldsSet: true,
	})
	if err != nil {
		t.Fatalf("set pf: %v", err)
	}

	updated, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		PublicFields:    []string{"avatar_url"},
		PublicFieldsSet: true,
	})
	if err != nil {
		t.Fatalf("replace pf: %v", err)
	}
	if len(updated.PublicFields) != 1 || updated.PublicFields[0] != "avatar_url" {
		t.Errorf("public_fields should be fully replaced, got %v", updated.PublicFields)
	}
}

func TestService_UpdateProfile_UserNotFound(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	_, err := svc.UpdateProfile(context.Background(), uuid.New(), UpdateProfileInput{
		Bio: "hello",
	})
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

// --- GetProfile ---

func TestService_GetProfile_Public(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "gp_pub")

	got, hidden, err := svc.GetProfile(context.Background(), reg.User.Username, uuid.Nil)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if hidden {
		t.Error("public profile must not be hidden")
	}
	if got.ID != reg.User.ID {
		t.Errorf("id mismatch")
	}
	if got.Username != reg.User.Username {
		t.Errorf("username mismatch")
	}
}

func TestService_GetProfile_Private_Anonymous(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "gp_anon")

	_, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		IsPrivate:       true,
		IsPrivateSet:    true,
		Bio:             "public bio",
		PublicFields:    []string{"bio"},
		PublicFieldsSet: true,
	})
	if err != nil {
		t.Fatalf("set private: %v", err)
	}

	got, hidden, err := svc.GetProfile(context.Background(), reg.User.Username, uuid.Nil)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if !hidden {
		t.Error("private profile must be hidden for anonymous viewer")
	}
	if got.ID != reg.User.ID {
		t.Error("id must still be exposed")
	}
	if got.Username != reg.User.Username {
		t.Error("username must still be exposed")
	}
	if got.Bio != "public bio" {
		t.Errorf("bio is in public_fields, must be exposed, got %q", got.Bio)
	}
	if got.FirstName != "" {
		t.Error("first_name is not in public_fields, must be filtered out")
	}
	if got.Email != "" {
		t.Error("email must NEVER be exposed in a hidden profile")
	}
}

func TestService_GetProfile_Private_Owner(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "gp_own")

	_, err := svc.UpdateProfile(context.Background(), reg.User.ID, UpdateProfileInput{
		IsPrivate:       true,
		IsPrivateSet:    true,
		PublicFields:    []string{"bio"},
		PublicFieldsSet: true,
	})
	if err != nil {
		t.Fatalf("set private: %v", err)
	}

	got, hidden, err := svc.GetProfile(context.Background(), reg.User.Username, reg.User.ID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if hidden {
		t.Error("owner must see full profile, hidden=false")
	}
	if got.Email != reg.User.Email {
		t.Errorf("owner must see email, got %q", got.Email)
	}
}

func TestService_GetProfile_Private_OtherUser(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	owner, _ := registerForTest(t, svc, db, "gp_priv_o")
	viewer, _ := registerForTest(t, svc, db, "gp_priv_v")

	_, err := svc.UpdateProfile(context.Background(), owner.User.ID, UpdateProfileInput{
		IsPrivate:       true,
		IsPrivateSet:    true,
		PublicFields:    []string{"bio"},
		PublicFieldsSet: true,
	})
	if err != nil {
		t.Fatalf("set private: %v", err)
	}

	got, hidden, err := svc.GetProfile(context.Background(), owner.User.Username, viewer.User.ID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if !hidden {
		t.Error("other user must see hidden profile for private owner")
	}
	if got.Email != "" {
		t.Error("email must be hidden from non-owner")
	}
}

func TestService_GetProfile_NotFound(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	_, _, err := svc.GetProfile(context.Background(), "nobody_"+uniqueSuffix(), uuid.Nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_GetProfile_Public_Anonymous_NoEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	reg, _ := registerForTest(t, svc, db, "gp_pub_noemail")

	got, hidden, err := svc.GetProfile(context.Background(), reg.User.Username, uuid.Nil)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if hidden {
		t.Error("public profile must not be hidden")
	}
	if got.ID != reg.User.ID {
		t.Error("id mismatch")
	}
	if got.Email != "" {
		t.Errorf("email must not be exposed in public profile, got %q", got.Email)
	}
}

func TestService_GetProfile_Public_OtherUser_NoEmail(t *testing.T) {
	svc, _, db := newTestService(t)
	defer db.Close()

	owner, _ := registerForTest(t, svc, db, "gp_pub_own")
	viewer, _ := registerForTest(t, svc, db, "gp_pub_view")

	got, _, err := svc.GetProfile(context.Background(), owner.User.Username, viewer.User.ID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if got.Email != "" {
		t.Errorf("email must not be exposed to another user, got %q", got.Email)
	}
}

var _ = models.User{}
