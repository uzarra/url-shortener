package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserIDFromContext_InvalidType(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey{}, 42)

	id, err := UserIDFromContext(ctx)

	assert.ErrorIs(t, err, ErrUserIDInvalidType)
	assert.Contains(t, err.Error(), "int")
	assert.Empty(t, id)
}
