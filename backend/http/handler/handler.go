package handler

import (
	"context"
	"errors"
	"log/slog"

	microerr "go.unistack.org/micro/v3/errors"

	pb "github.com/VT0x00/vyborok/http/proto"
	"github.com/VT0x00/vyborok/internal/auth"
	"github.com/VT0x00/vyborok/internal/models"
	"github.com/google/uuid"
)

type VyborokHandler struct {
	auth   *auth.Service
	logger *slog.Logger
}

func New(authSvc *auth.Service, logger *slog.Logger) *VyborokHandler {
	return &VyborokHandler{auth: authSvc, logger: logger}
}

func (h *VyborokHandler) Health(ctx context.Context, req *pb.HealthReq, rsp *pb.HealthRsp) error {
	rsp.Status = "ok"
	return nil
}

func (h *VyborokHandler) Register(ctx context.Context, req *pb.RegisterReq, rsp *pb.RegisterRsp) error {
	res, err := h.auth.Register(ctx, auth.RegisterInput{
		Email:     req.Email,
		Password:  req.Password,
		Username:  req.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		h.logger.Error("register failed", "err", err, "email", req.Email, "username", req.Username)
		return mapAuthError(err)
	}

	rsp.AccessToken = res.AccessToken
	rsp.RefreshToken = res.RefreshToken
	rsp.Profile = toProfile(res.User)
	return nil
}

func (h *VyborokHandler) Login(ctx context.Context, req *pb.LoginReq, rsp *pb.LoginRsp) error {
	res, err := h.auth.Login(ctx, req.Email, req.Password)
	if err != nil {
		h.logger.Error("login failed", "err", err, "email", req.Email)
		return mapAuthError(err)
	}

	rsp.AccessToken = res.AccessToken
	rsp.RefreshToken = res.RefreshToken
	rsp.Profile = toProfile(res.User)
	return nil
}

func (h *VyborokHandler) GetMe(ctx context.Context, req *pb.GetMeReq, rsp *pb.ProfileRsp) error {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return microerr.Unauthorized("unauthorized", "auth required")
	}

	user, err := h.auth.GetMe(ctx, userID)
	if err != nil {
		h.logger.Error("get me failed", "err", err, "user_id", userID)
		return mapAuthError(err)
	}

	rsp.Profile = toProfile(user)
	return nil
}

func (h *VyborokHandler) Refresh(ctx context.Context, req *pb.RefreshReq, rsp *pb.LoginRsp) error {
	res, err := h.auth.Refresh(ctx, req.RefreshToken)
	if err != nil {
		h.logger.Error("refresh failed", "err", err)
		return mapAuthError(err)
	}

	rsp.AccessToken = res.AccessToken
	rsp.RefreshToken = res.RefreshToken
	rsp.Profile = toProfile(res.User)
	return nil
}

func (h *VyborokHandler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileReq, rsp *pb.ProfileRsp) error {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return microerr.Unauthorized("unauthorized", "auth required")
	}

	user, err := h.auth.UpdateProfile(ctx, userID, auth.UpdateProfileInput{
		Username:        req.Username,
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		Bio:             req.Bio,
		Links:           req.Links,
		AvatarURL:       req.AvatarUrl,
		IsPrivate:       req.IsPrivate,
		IsPrivateSet:    req.IsPrivateSet,
		PublicFields:    req.PublicFields,
		PublicFieldsSet: req.PublicFieldsSet,
	})
	if err != nil {
		h.logger.Error("update profile failed", "err", err, "user_id", userID)
		return mapAuthError(err)
	}

	rsp.Profile = toProfile(user)
	return nil
}

func (h *VyborokHandler) GetProfile(ctx context.Context, req *pb.GetProfileReq, rsp *pb.ProfileRsp) error {
	var viewerID uuid.UUID
	if id, ok := auth.UserIDFromContext(ctx); ok {
		viewerID = id
	}

	user, hidden, err := h.auth.GetProfile(ctx, req.Username, viewerID)
	if err != nil {
		h.logger.Error("get profile failed", "err", err, "username", req.Username)
		return mapAuthError(err)
	}

	rsp.Profile = toProfile(user)
	rsp.ProfileHidden = hidden
	return nil
}

func toProfile(u *models.User) *pb.UserProfile {
	if u == nil {
		return nil
	}
	return &pb.UserProfile{
		Id:           u.ID.String(),
		Username:     u.Username,
		Email:        u.Email,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Bio:          u.Bio,
		Links:        u.Links,
		AvatarUrl:    u.AvatarURL,
		IsPrivate:    u.IsPrivate,
		PublicFields: u.PublicFields,
	}
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, auth.ErrEmailTaken):
		return microerr.BadRequest("email_taken", "email already registered")
	case errors.Is(err, auth.ErrUsernameTaken):
		return microerr.BadRequest("username_taken", "username already taken")
	case errors.Is(err, auth.ErrInvalidCredentials):
		return microerr.Unauthorized("invalid_credentials", "invalid email or password")
	case errors.Is(err, auth.ErrInvalidToken):
		return microerr.Unauthorized("invalid_token", "invalid or expired token")
	case errors.Is(err, auth.ErrNotFound):
		return microerr.NotFound("not_found", "user not found")
	case errors.Is(err, auth.ErrInvalidInput):
		return microerr.BadRequest("invalid_input", "%s", err.Error())
	default:
		return microerr.InternalServerError("internal_error", "something went wrong")
	}
}

var _ pb.VyborokServer = (*VyborokHandler)(nil)
