package auth_test

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uzarra/url-shortener/internal/auth"
)

func TestNewUserID_IsUUIDv4(t *testing.T) {
	id, err := uuid.Parse(auth.NewUserID())
	require.NoError(t, err)

	assert.Equal(t, byte(4), id[6]>>4, "version must be 4")
	assert.Equal(t, byte(0x80), id[8]&0xc0, "variant must be RFC 9562")
}

func TestNewUserID_Unique(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for range n {
		id := auth.NewUserID()
		_, dup := seen[id]
		require.False(t, dup, "duplicate id %s", id)
		seen[id] = struct{}{}
	}
}
