package handler

import (
	"context"
	"errors"
	"log/slog"

	microerr "go.unistack.org/micro/v3/errors"

	pb "github.com/VT0x00/vyborok/http/proto"
	"github.com/VT0x00/vyborok/internal/auth"
	"github.com/VT0x00/vyborok/internal/models"
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

func toProfile(u *models.User) *pb.UserProfile {
	if u == nil {
		return nil
	}
	return &pb.UserProfile{
		Id:        u.ID.String(),
		Username:  u.Username,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		AvatarUrl: u.AvatarURL,
		IsPrivate: u.IsPrivate,
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
	case errors.Is(err, auth.ErrInvalidInput):
		return microerr.BadRequest("invalid_input", err.Error())
	default:
		return microerr.InternalServerError("internal_error", "something went wrong")
	}
}

var _ pb.VyborokServer = (*VyborokHandler)(nil)
