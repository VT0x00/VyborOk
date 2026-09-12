package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/VT0x00/vyborok/internal/models"
	"github.com/VT0x00/vyborok/internal/repository"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailTaken         = errors.New("email already taken")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

var (
	emailRE    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)
)

type Service struct {
	users *repository.UserRepository
	jwt   *JWTManager
}

func NewService(users *repository.UserRepository, jwt *JWTManager) *Service {
	return &Service{users: users, jwt: jwt}
}

type RegisterInput struct {
	Email     string
	Password  string
	Username  string
	FirstName string
	LastName  string
}

type AuthResult struct {
	User         *models.User
	AccessToken  string
	RefreshToken string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.Username = strings.TrimSpace(in.Username)
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.LastName = strings.TrimSpace(in.LastName)

	if err := validateRegister(in); err != nil {
		return nil, err
	}

	exists, err := s.users.ExistsByEmail(ctx, in.Email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, ErrEmailTaken
	}

	exists, err = s.users.ExistsByUsername(ctx, in.Username)
	if err != nil {
		return nil, fmt.Errorf("check username: %w", err)
	}
	if exists {
		return nil, ErrUsernameTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		Email:        in.Email,
		PasswordHash: string(hash),
		Username:     in.Username,
		FirstName:    in.FirstName,
		LastName:     in.LastName,
		Links:        []string{},
		PublicFields: []string{},
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	access, refresh, err := s.jwt.Generate(user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Не раскрываем, что именно неверно: email или пароль.
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	access, refresh, err := s.jwt.Generate(user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, AccessToken: access, RefreshToken: refresh}, nil
}

func validateRegister(in RegisterInput) error {
	if !emailRE.MatchString(in.Email) {
		return fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if len(in.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 chars", ErrInvalidInput)
	}
	if len(in.Password) > 72 {
		// bcrypt молча обрезает пароль после 72 байт, поэтому лучше явно ограничить.
		return fmt.Errorf("%w: password too long", ErrInvalidInput)
	}
	if !usernameRE.MatchString(in.Username) {
		return fmt.Errorf("%w: username must be 3-30 chars, letters/digits/underscore", ErrInvalidInput)
	}
	return nil
}
