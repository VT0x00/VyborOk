package auth

import (
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Claims struct {
	UserID uuid.UUID `json:"uid"`
	Type   TokenType `json:"typ"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secret, issuer string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m *JWTManager) Generate(userID uuid.UUID) (access, refresh string, err error) {
	access, err = m.sign(userID, TokenTypeAccess, m.accessTTL)
	if err != nil {
		return "", "", fmt.Errorf("sign access: %w", err)
	}
	refresh, err = m.sign(userID, TokenTypeRefresh, m.refreshTTL)
	if err != nil {
		return "", "", fmt.Errorf("sign refresh: %w", err)
	}

	return access, refresh, nil
}

func (m *JWTManager) Parse(tokenStr string, expected TokenType) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.Type != expected {
		return nil, fmt.Errorf("wrong token type: got %s, want %s", claims.Type, expected)
	}

	return claims, nil
}

func (m *JWTManager) sign(userID uuid.UUID, t TokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Type:   t,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(m.secret)
}
