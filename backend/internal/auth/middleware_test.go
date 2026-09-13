package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	microerr "go.unistack.org/micro/v3/errors"
	"go.unistack.org/micro/v3/metadata"
	"go.unistack.org/micro/v3/server"
)

type mockRequest struct {
	server.Request
	headers metadata.Metadata
}

func (m *mockRequest) Header() metadata.Metadata {
	return m.headers
}

func newJWTManager() *JWTManager {
	return NewJWTManager("test-secret", "vyborok-test", time.Minute, time.Hour)
}

func runMiddleware(m *JWTManager, headers metadata.Metadata, next server.FuncHandler) (error, bool) {
	called := false
	countingNext := func(ctx context.Context, req server.Request, rsp interface{}) error {
		called = true
		if next != nil {
			return next(ctx, req, rsp)
		}
		return nil
	}

	hook := m.Middleware()
	wrapped := hook(countingNext)
	req := &mockRequest{headers: headers}
	err := wrapped(context.Background(), req, nil)
	return err, called
}

func TestMiddleware_NoHeader(t *testing.T) {
	m := newJWTManager()

	err, called := runMiddleware(m, metadata.Metadata{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	m := newJWTManager()
	uid := uuid.New()
	access, _, _ := m.Generate(uid)

	var gotUID uuid.UUID
	var gotOK bool
	next := func(ctx context.Context, req server.Request, rsp interface{}) error {
		gotUID, gotOK = UserIDFromContext(ctx)
		return nil
	}

	err, called := runMiddleware(m, metadata.Metadata{
		"Authorization": "Bearer " + access,
	}, next)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
	if !gotOK {
		t.Fatal("user id was not put into context")
	}
	if gotUID != uid {
		t.Errorf("user id mismatch: got %v, want %v", gotUID, uid)
	}
}

func TestMiddleware_WrongScheme(t *testing.T) {
	m := newJWTManager()

	err, called := runMiddleware(m, metadata.Metadata{
		"Authorization": "Basic dXNlcjpwYXNz",
	}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if called {
		t.Fatal("next handler must not be called")
	}

	var merr *microerr.Error
	if !errors.As(err, &merr) || merr.Code != 401 {
		t.Errorf("expected 401, got %v", err)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	m := newJWTManager()

	err, called := runMiddleware(m, metadata.Metadata{
		"Authorization": "Bearer not-a-token",
	}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if called {
		t.Fatal("next handler must not be called")
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	m := NewJWTManager("test-secret", "vyborok-test", -1*time.Second, time.Hour)
	uid := uuid.New()
	access, _, _ := m.Generate(uid)

	time.Sleep(10 * time.Millisecond)

	err, called := runMiddleware(m, metadata.Metadata{
		"Authorization": "Bearer " + access,
	}, nil)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if called {
		t.Fatal("next handler must not be called")
	}
}

func TestMiddleware_LowercaseHeader(t *testing.T) {
	m := newJWTManager()
	uid := uuid.New()
	access, _, _ := m.Generate(uid)

	var gotUID uuid.UUID
	var gotOK bool
	next := func(ctx context.Context, req server.Request, rsp interface{}) error {
		gotUID, gotOK = UserIDFromContext(ctx)
		return nil
	}

	err, called := runMiddleware(m, metadata.Metadata{
		"authorization": "Bearer " + access,
	}, next)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
	if !gotOK || gotUID != uid {
		t.Errorf("user id was not put into context correctly")
	}
}

func TestMiddleware_RefreshTokenRejected(t *testing.T) {
	m := newJWTManager()
	uid := uuid.New()

	_, refresh, err := m.Generate(uid)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	err, called := runMiddleware(m, metadata.Metadata{
		"Authorization": "Bearer " + refresh,
	}, nil)
	if err == nil {
		t.Fatal("expected error for refresh token used as access, got nil")
	}
	if called {
		t.Fatal("next handler must not be called")
	}

	var merr *microerr.Error
	if !errors.As(err, &merr) || merr.Code != 401 {
		t.Errorf("expected 401, got %v", err)
	}
}

func TestMiddleware_EmptyBearerToken(t *testing.T) {
	m := newJWTManager()

	err, called := runMiddleware(m, metadata.Metadata{
		"Authorization": "Bearer ",
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty bearer token, got nil")
	}
	if called {
		t.Fatal("next handler must not be called")
	}

	var merr *microerr.Error
	if !errors.As(err, &merr) || merr.Code != 401 {
		t.Errorf("expected 401, got %v", err)
	}
}