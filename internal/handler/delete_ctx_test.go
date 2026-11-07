package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestHandler_DeleteUserURLs_ClientGone(t *testing.T) {
	for _, ctxErr := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(ctxErr.Error(), func(t *testing.T) {
			var logBuf bytes.Buffer
			log := zerolog.New(&logBuf).Level(zerolog.InfoLevel)
			h := New(newTestShortener(), &testDeleter{err: ctxErr}, &testPinger{}, log)

			req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", strings.NewReader(`["abc"]`))
			req.Header.Set(headerContentType, "application/json")
			rec := httptest.NewRecorder()

			h.DeleteUserURLs(rec, withUser(req, handlerTestUser))

			assert.NotEqual(t, http.StatusServiceUnavailable, rec.Code)
			assert.Empty(t, logBuf.String(), "client disconnect must not be logged as error")
		})
	}
}
