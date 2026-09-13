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

func newTestMiddleware(t *testing.T) (*JWTManager, server.HandlerFunc, *bool) {
	t.Helper()
	m := NewJWTManager("test-secret", "vyborok-test", time.Minute, time.Hour)
	called := false
	next := func(ctx context.Context, req server.Request, rsp any) error {
		called = true
		return nil
	}
	return m, next, &called
}

func TestMiddleware_NoHeader(t *testing.T) {
	m, next, called := newTestMiddleware(t)
	mw := m.Middleware(next)

	req := &mockRequest{headers: metadata.Metadata{}}
	err := mw(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*called {
		t.Fatal("next handler was not called")
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	m, next, called := newTestMiddleware(t)
	mw := m.Middleware(next)

	uid := uuid.New()
	access, _, _ := m.Generate(uid)

	var gotUID uuid.UUID
	var gotOK bool
	nextWithCapture := func(ctx context.Context, req server.Request, rsp any) error {
		*called = true
		gotUID, gotOK = UserIDFromContext(ctx)
		return nil
	}
	mw = m.Middleware(nextWithCapture)

	req := &mockRequest{headers: metadata.Metadata{
		"Authorization": "Bearer " + access,
	}}
	if err := mw(context.Background(), req, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*called {
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
	m, next, _ := newTestMiddleware(t)
	mw := m.Middleware(next)

	req := &mockRequest{headers: metadata.Metadata{
		"Authorization": "Basic dXNlcjpwYXNz",
	}}
	err := mw(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var merr *microerr.Error
	if !errors.As(err, &merr) || merr.Code != 401 {
		t.Errorf("expected 401, got %v", err)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	m, next, _ := newTestMiddleware(t)
	mw := m.Middleware(next)

	req := &mockRequest{headers: metadata.Metadata{
		"Authorization": "Bearer not-a-token",
	}}
	err := mw(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	m := NewJWTManager("test-secret", "vyborok-test", -1*time.Second, time.Hour)
	called := false
	next := func(ctx context.Context, req server.Request, rsp any) error {
		called = true
		return nil
	}
	mw := m.Middleware(next)

	uid := uuid.New()
	access, _, _ := m.Generate(uid)
	time.Sleep(10 * time.Millisecond)

	req := &mockRequest{headers: metadata.Metadata{
		"Authorization": "Bearer " + access,
	}}
	err := mw(context.Background(), req, nil)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if called {
		t.Fatal("next handler must not be called")
	}
}

func TestMiddleware_LowercaseHeader(t *testing.T) {
	m, next, called := newTestMiddleware(t)
	mw := m.Middleware(next)

	uid := uuid.New()
	access, _, _ := m.Generate(uid)

	req := &mockRequest{headers: metadata.Metadata{
		"authorization": "Bearer " + access,
	}}
	if err := mw(context.Background(), req, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*called {
		t.Fatal("next handler was not called")
	}
}
