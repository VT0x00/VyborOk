package auth

import (
	"context"

	"github.com/google/uuid"
)

// ctxKey — приватный тип для ключей контекста.
// Экспортировать нельзя: если другой пакет определит свой ctxKey с тем же
// значением, произойдёт коллизия. Приватный тип исключает такое.
type ctxKey int

const userIDKey ctxKey = 1

// withUserID кладёт ID пользователя в контекст.
// Приватная — снаружи должен использоваться только UserIDFromContext.
func withUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserIDFromContext достаёт ID пользователя из контекста.
// Второй возвращаемый параметр — false, если ID нет (запрос анонимный).
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}
