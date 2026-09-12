package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/VT0x00/vyborok/internal/models"
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

func newUser() *models.User {
	suffix := uuid.NewString()[:8]
	return &models.User{
		Email:        "test_" + suffix + "@example.com",
		PasswordHash: "$2a$10$dummyhashforvborok",
		Username:     "user_" + suffix,
		FirstName:    "Тест",
		LastName:     "Тестович",
		Bio:          "Привет, я Тестовый пользователь",
		Links:        []string{"https://github.com/test"},
		IsPrivate:    false,
		PublicFields: []string{"username", "bio"},
	}
}

func cleanup(t *testing.T, db *sql.DB, id uuid.UUID) {
	t.Helper()
	_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, id)
}

func TestUserRepository_CreateAndGetByID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	u := newUser()
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer cleanup(t, db, u.ID)

	if u.ID == uuid.Nil {
		t.Fatal("expected non-nil ID after create")
	}
	if u.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}

	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}

	if got.Email != u.Email {
		t.Errorf("email mismatch: got %q, want %q", got.Email, u.Email)
	}
	if got.Username != u.Username {
		t.Errorf("username mismatch: got %q, want %q", got.Username, u.Username)
	}
	if len(got.Links) != 1 || got.Links[0] != u.Links[0] {
		t.Errorf("links mismatch: got %v, want %v", got.Links, u.Links)
	}
	if len(got.PublicFields) != 2 {
		t.Errorf("public_fields length mismatch: got %d, want 2", len(got.PublicFields))
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	u := newUser()
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer cleanup(t, db, u.ID)

	got, err := repo.GetByEmail(ctx, u.Email)
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("id mismatch: got %v, want %v", got.ID, u.ID)
	}
}

func TestUserRepository_GetByUsername(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	u := newUser()
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer cleanup(t, db, u.ID)

	got, err := repo.GetByUsername(ctx, u.Username)
	if err != nil {
		t.Fatalf("get by username: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("id mismatch: got %v, want %v", got.ID, u.ID)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	_, err := repo.GetByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUserRepository_ExistsByEmail(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	u := newUser()
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer cleanup(t, db, u.ID)

	exists, err := repo.ExistsByEmail(ctx, u.Email)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !exists {
		t.Error("expected user to exist")
	}

	exists, err = repo.ExistsByEmail(ctx, "nonexistent_"+uuid.NewString()+"@example.com")
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if exists {
		t.Error("expected user not to exist")
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	u := newUser()
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer cleanup(t, db, u.ID)

	u.FirstName = "Пётр"
	u.Bio = "Обновлённая биография"
	u.IsPrivate = true
	u.Links = []string{"https://t.me/vt0x00", "https://github.com/vt0x00"}

	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Небольшая задержка — updated_at может совпасть с created_at, если
	// PostgreSQL не успел обновить время. В реальном коде это не нужно.
	time.Sleep(10 * time.Millisecond)

	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}

	if got.FirstName != "Пётр" {
		t.Errorf("first_name mismatch: got %q", got.FirstName)
	}
	if got.Bio != "Обновлённая биография" {
		t.Errorf("bio mismatch: got %q", got.Bio)
	}
	if !got.IsPrivate {
		t.Error("expected is_private = true")
	}
	if len(got.Links) != 2 {
		t.Errorf("links length mismatch: got %d, want 2", len(got.Links))
	}
}

func TestUserRepository_Delete(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)
	ctx := context.Background()

	u := newUser()
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := repo.GetByID(ctx, u.ID)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
