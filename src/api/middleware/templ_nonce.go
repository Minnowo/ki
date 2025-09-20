package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
)

func TemplNonce(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var nonceBytes [32]byte
		rand.Read(nonceBytes[:])

		nonce := hex.EncodeToString(nonceBytes[:])

		w.Header().Add("Content-Security-Policy", fmt.Sprintf("script-src 'nonce-%s'", nonce))

		// Use the context to pass the nonce to the handler.
		ctx := templ.WithNonce(r.Context(), nonce)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
