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
		name    string
		ctx     context.Context
		wantID  string
		wantErr error
	}{
		{
			name:   "user id set",
			ctx:    auth.WithUserID(context.Background(), userID),
			wantID: userID,
		},
		{
			name:    "empty context",
			ctx:     context.Background(),
			wantErr: auth.ErrUserIDNotFound,
		},
		{
			name:    "empty user id",
			ctx:     auth.WithUserID(context.Background(), ""),
			wantErr: auth.ErrEmptyUserID,
		},
		{
			name:    "foreign key from another package does not collide",
			ctx:     context.WithValue(context.Background(), foreignKey("userID"), userID),
			wantErr: auth.ErrUserIDNotFound,
		},
		{
			name:   "overwritten by later value",
			ctx:    auth.WithUserID(auth.WithUserID(context.Background(), "old"), userID),
			wantID: userID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := auth.UserIDFromContext(tt.ctx)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantID, id)
		})
	}
}
