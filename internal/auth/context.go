package auth

import (
	"context"
	"fmt"
)

type ctxKey struct{}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}

func UserIDFromContext(ctx context.Context) (string, error) {
	v := ctx.Value(ctxKey{})
	if v == nil {
		return "", ErrUserIDNotFound
	}
	id, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%w: got %T", ErrUserIDInvalidType, v)
	}
	if id == "" {
		return "", ErrEmptyUserID
	}
	return id, nil
}
