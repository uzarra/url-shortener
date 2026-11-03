package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uzarra/url-shortener/internal/auth"
)

type foreignKey string

func TestUserIDFromContext(t *testing.T) {
	userID := auth.NewUserID()

	tests := []struct {
		name   string
		ctx    context.Context
		wantID string
		wantOK bool
	}{
		{
			name:   "user id set",
			ctx:    auth.WithUserID(context.Background(), userID),
			wantID: userID,
			wantOK: true,
		},
		{
			name:   "empty context",
			ctx:    context.Background(),
			wantID: "",
			wantOK: false,
		},
		{
			name:   "empty user id",
			ctx:    auth.WithUserID(context.Background(), ""),
			wantID: "",
			wantOK: false,
		},
		{
			name:   "foreign key from another package does not collide",
			ctx:    context.WithValue(context.Background(), foreignKey("userID"), userID),
			wantID: "",
			wantOK: false,
		},
		{
			name:   "overwritten by later value",
			ctx:    auth.WithUserID(auth.WithUserID(context.Background(), "old"), userID),
			wantID: userID,
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := auth.UserIDFromContext(tt.ctx)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantID, id)
		})
	}
}
