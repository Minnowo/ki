package middleware

import (
	"context"
	"ki/src/api"
	"ki/src/config"
	"ki/src/handlers/user"
	"net/http"

	"github.com/rs/zerolog/log"
)

// If the user provides a valid auth token, the username is set in the context using the config.USER_CONTEXT_KEY
func Auth(userReg *user.UserRegistry) func(next http.Handler) http.Handler {
	return ParseAuth(config.USER_CONTEXT_KEY, userReg)
}

// If the user provides a valid auth token, the username is set in the context using the given user key
func ParseAuth(userKey string, userReg *user.UserRegistry) func(next http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			authToken, err := r.Cookie(config.SESSION_COOKIE)

			if err == nil {

				username, ok := userReg.CheckToken(authToken.Value)

				if ok {
					log.Debug().Str("user", username).Msg("valid session")

					ctx := context.WithValue(context.Background(), userKey, username)

					r = r.WithContext(ctx)
				} else {
					log.Debug().Msg("expired session")

					authToken.MaxAge = -1
					authToken.Value = ""
					http.SetCookie(w, authToken)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// respond with unauthorized if there was no config.USER_CONTEXT_KEY value set in the context
func RequireAuth() func(next http.Handler) http.Handler {
	return RequireContextString(config.USER_CONTEXT_KEY)
}

// respond with unauthorized if there was no userKey value set in the context
func RequireContextString(userKey string) func(next http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			_, ok := r.Context().Value(userKey).(string)

			if !ok {
				api.Unauthorized(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
