package auth

import (
	"context"
	"ki/src/api"
	"ki/src/handlers/user"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	ctxUserKey = struct{ name string }{name: "user"}
)

// ParseAuth parses the session cookie into a user from the userReg and adds it to the request.
func ParseAuth(sessionCookie string, userReg *user.UserRegistry) func(next http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			authToken, err := r.Cookie(sessionCookie)

			if err == nil {

				username, ok := userReg.CheckToken(authToken.Value)

				if ok {
					log.Debug().Str("user", username).Msg("valid session")

					ctx := context.WithValue(r.Context(), ctxUserKey, username)

					r = r.WithContext(ctx)
				} else {
					log.Debug().Msg("expired session")

					authToken.Expires = time.Unix(0, 0)
					authToken.MaxAge = -1
					authToken.Value = ""
					http.SetCookie(w, authToken)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth requires that the request has a valid user, otherwise it returns a 401.
func RequireAuth() func(next http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if !IsAuthed(r) {
				api.Unauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUser returns the user authenticated for this request, if available
func GetUser(r *http.Request) (string, bool) {

	user, ok := r.Context().Value(ctxUserKey).(string)

	if !ok {
		return "", false
	}

	return user, true
}

// IsAuthed returns if the request has a valid user
func IsAuthed(r *http.Request) bool {

	_, ok := GetUser(r)

	log.Debug().Bool("ok", ok).Msg("IsAuthed")

	return ok
}
