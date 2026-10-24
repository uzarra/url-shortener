package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapInsertError(t *testing.T) {
	errDB := errors.New("db is down")
	errNotNull := &pgconn.PgError{Code: pgerrcode.NotNullViolation}
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "original url conflict",
			err:  &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: originalURLUniqueIndex},
			want: ErrConflict},
		{name: "short url collision",
			err:  &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "urls_short_url_key"},
			want: ErrAlreadyExists},
		{name: "other pg error", err: errNotNull, want: errNotNull},
		{name: "other error", err: errDB, want: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapInsertError(tt.err); !errors.Is(got, tt.want) {
				t.Errorf("mapInsertError() = %v, want %v", got, tt.want)
			}
		})
	}
}
