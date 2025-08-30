package tracking

import "net/http"

func IsBehindCloudflare(r *http.Request) bool {
	return r.Header.Get("CF-Ray") != "" || r.Header.Get("CF-Connecting-IP") != ""
}
