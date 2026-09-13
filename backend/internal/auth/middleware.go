package auth

import (
	"context"
	"strings"

	"go.unistack.org/micro/v3/server"

	microerr "go.unistack.org/micro/v3/errors"
)

const bearerPrefix = "Bearer "

// Middleware опознаёт пользователя по JWT и кладёт userID в контекст.
// Возвращает server.HookHandler, чтобы micro смог применить его как hook.
func (m *JWTManager) Middleware() server.HookHandler {
	return func(next server.FuncHandler) server.FuncHandler {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			md := req.Header()

			authz := md["Authorization"]
			if authz == "" {
				authz = md["authorization"]
			}

			if authz == "" {
				return next(ctx, req, rsp)
			}

			if !strings.HasPrefix(authz, bearerPrefix) {
				return microerr.Unauthorized("invalid_token", "expected Bearer scheme")
			}

			token := strings.TrimPrefix(authz, bearerPrefix)
			claims, err := m.Parse(token, TokenTypeAccess)
			if err != nil {
				return microerr.Unauthorized("invalid_token", "%s", err.Error())
			}

			ctx = withUserID(ctx, claims.UserID)
			return next(ctx, req, rsp)
		}
	}
}
