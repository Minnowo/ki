package request

import (
	"net/http"
)

// IsTLS determines if this request should be considered TLS enabled
func IsTLS(r *http.Request) bool {

	if r.URL.Scheme == "" {
		return r.TLS != nil
	}

	// URL.Schema will be set by middleware, if behind a proxy
	return r.URL.Scheme == "https"
}
