package middleware

import (
	"errors"
	"net/http"

	"github.com/uzarra/url-shortener/internal/auth"
)

const cookieName = "url-shortener.token"

func Auth(m *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err == nil {
				userID, parseErr := m.Parse(cookie.Value)
				if parseErr == nil {
					r = r.WithContext(auth.WithUserID(r.Context(), userID))
					next.ServeHTTP(w, r)
					return
				}
				if errors.Is(parseErr, auth.ErrNoUserID) {
					next.ServeHTTP(w, r)
					return
				}
			}
			userID := auth.NewUserID()
			token, err := m.Build(userID)
			if err != nil {
				http.Error(w, "failed to issue auth token", http.StatusInternalServerError)
				return
			}
			http.SetCookie(
				w,
				&http.Cookie{
					Name:     cookieName,
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
					MaxAge:   int(auth.TokenTTL.Seconds()),
				},
			)
			r = r.WithContext(auth.WithUserID(r.Context(), userID))
			next.ServeHTTP(w, r)
		})
	}
}
