package form

import (
	"net/http"
	"strconv"
	"strings"
)

func ReadInt32(r *http.Request, out *int32, field string) bool {

	s := strings.TrimSpace(r.FormValue(field))

	v, err := strconv.ParseInt(s, 10, 32)

	if err != nil {
		return false
	}

	*out = int32(v)

	return true
}
