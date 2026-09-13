package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/VT0x00/vyborok/internal/models"
	"github.com/VT0x00/vyborok/internal/repository"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailTaken         = errors.New("email already taken")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrNotFound           = errors.New("not found")
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

type UpdateProfileInput struct {
	Username        string
	FirstName       string
	LastName        string
	Bio             string
	Links           []string
	AvatarURL       string
	IsPrivate       bool
	IsPrivateSet    bool
	PublicFields    []string
	PublicFieldsSet bool
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

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *Service) GetMe(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	if refreshToken == "" {
		return nil, ErrInvalidInput
	}

	claims, err := s.jwt.Parse(refreshToken, TokenTypeRefresh)
	if err != nil {
		return nil, ErrInvalidToken
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	access, refresh, err := s.jwt.Generate(user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResult{User: user, AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (*models.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if in.Username != "" && in.Username != user.Username {
		if !usernameRE.MatchString(in.Username) {
			return nil, fmt.Errorf("%w: invalid username", ErrInvalidInput)
		}
		other, err := s.users.GetByUsername(ctx, in.Username)
		if err == nil && other.ID != user.ID {
			return nil, ErrUsernameTaken
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("check username: %w", err)
		}
		user.Username = in.Username
	}

	if in.FirstName != "" {
		user.FirstName = in.FirstName
	}
	if in.LastName != "" {
		user.LastName = in.LastName
	}
	if in.Bio != "" {
		user.Bio = in.Bio
	}
	if in.AvatarURL != "" {
		user.AvatarURL = in.AvatarURL
	}
	if in.Links != nil {
		user.Links = in.Links
	}
	if in.IsPrivateSet {
		user.IsPrivate = in.IsPrivate
	}
	if in.PublicFieldsSet {
		user.PublicFields = in.PublicFields
	}

	if err := s.users.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

func (s *Service) GetProfile(ctx context.Context, username string, viewerID uuid.UUID) (*models.User, bool, error) {
	user, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, false, ErrNotFound
		}
		return nil, false, fmt.Errorf("get user: %w", err)
	}

	if !user.IsPrivate {
		return user, false, nil
	}

	if viewerID != uuid.Nil && viewerID == user.ID {
		return user, false, nil
	}

	return filterPublicFields(user), true, nil
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

func filterPublicFields(u *models.User) *models.User {
	out := &models.User{
		ID:           u.ID,
		Username:     u.Username,
		IsPrivate:    true,
		PublicFields: u.PublicFields,
	}
	for _, f := range u.PublicFields {
		switch f {
		case "first_name":
			out.FirstName = u.FirstName
		case "last_name":
			out.LastName = u.LastName
		case "bio":
			out.Bio = u.Bio
		case "links":
			out.Links = u.Links
		case "avatar_url":
			out.AvatarURL = u.AvatarURL
		}
	}
	return out
}
