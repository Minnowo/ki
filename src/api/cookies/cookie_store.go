package cookies

import (
	"fmt"
	"ki/src/pkg/csrf"
	"ki/src/pkg/request"
	"net/http"
	"time"
)

const (
	csrfCookiePath string        = "/"
	csrfMaxAge     time.Duration = time.Hour * 1
)

var (
	errInvalidToken = fmt.Errorf("token type is invalid")
)

type BasicCookieStore struct {
	CookieName string
}

// Load retrieves the CSRF token from the request's cookies
func (s BasicCookieStore) Load(r *http.Request, token any) error {

	cookie, err := r.Cookie(s.CookieName)

	if err != nil {
		return err
	}

	ptr, ok := token.(*[]byte)

	if !ok {
		return errInvalidToken
	}

	bytes, err := csrf.DecodeToken(cookie.Value)

	if err != nil {
		return err
	}

	*ptr = bytes

	return nil
}

// Save stores the CSRF token in the response's cookies
func (s BasicCookieStore) Save(w http.ResponseWriter, r *http.Request, token any) error {

	var tokenStr string

	ptr, ok := token.([]byte)

	if !ok {
		return errInvalidToken
	}

	tokenStr = csrf.EncodeToken(ptr)

	http.SetCookie(w, &http.Cookie{
		Name:     s.CookieName,
		Value:    tokenStr,
		Path:     csrfCookiePath,
		HttpOnly: true,
		Secure:   request.IsTLS(r),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(csrfMaxAge.Seconds()),
		Expires:  time.Now().Add(csrfMaxAge),
	})

	return nil
}
