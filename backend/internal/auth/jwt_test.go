package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestJWTManager() *JWTManager {
	return NewJWTManager("test-secret", "vyborok-test", 15*time.Minute, 24*time.Hour)
}

func TestJWTManager_GenerateAndParseAccess(t *testing.T) {
	m := newTestJWTManager()
	uid := uuid.New()

	access, refresh, err := m.Generate(uid)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("expected non-empty tokens")
	}
	if access == refresh {
		t.Fatal("access and refresh must differ")
	}

	claims, err := m.Parse(access, TokenTypeAccess)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.UserID != uid {
		t.Errorf("user id mismatch: got %v, want %v", claims.UserID, uid)
	}
	if claims.Type != TokenTypeAccess {
		t.Errorf("type mismatch: got %s", claims.Type)
	}
	if claims.Issuer != "vyborok-test" {
		t.Errorf("issuer mismatch: got %s", claims.Issuer)
	}
}

func TestJWTManager_ParseRefresh(t *testing.T) {
	m := newTestJWTManager()
	uid := uuid.New()

	_, refresh, err := m.Generate(uid)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := m.Parse(refresh, TokenTypeRefresh)
	if err != nil {
		t.Fatalf("parse refresh: %v", err)
	}
	if claims.UserID != uid {
		t.Errorf("user id mismatch")
	}
}

func TestJWTManager_Parse_WrongType(t *testing.T) {
	m := newTestJWTManager()
	uid := uuid.New()

	access, _, _ := m.Generate(uid)

	// attempting to parse the access token as a refresh token
	_, err := m.Parse(access, TokenTypeRefresh)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestJWTManager_Parse_Expired(t *testing.T) {
	// create a manager with an already expired access TTL
	m := NewJWTManager("test-secret", "vyborok-test", -1*time.Second, 24*time.Hour)
	uid := uuid.New()

	access, _, err := m.Generate(uid)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// a brief pause to ensure time has moved forward
	time.Sleep(10 * time.Millisecond)

	_, err = m.Parse(access, TokenTypeAccess)
	if err == nil {
		t.Fatal("expected expired error, got nil")
	}
}

func TestJWTManager_Parse_WrongSecret(t *testing.T) {
	m1 := NewJWTManager("secret-1", "vyborok-test", time.Minute, time.Hour)
	m2 := NewJWTManager("secret-2", "vyborok-test", time.Minute, time.Hour)

	access, _, _ := m1.Generate(uuid.New())

	_, err := m2.Parse(access, TokenTypeAccess)
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestJWTManager_Parse_Garbage(t *testing.T) {
	m := newTestJWTManager()

	_, err := m.Parse("not-a-token", TokenTypeAccess)
	if err == nil {
		t.Fatal("expected error for garbage, got nil")
	}
	_, err = m.Parse("", TokenTypeAccess)
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}
