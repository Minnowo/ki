package request

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func ParseRangeHeader(r *http.Request) (start, end int64, err error) {

	header := r.Header.Get("Range")

	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, fmt.Errorf("invalid Range header")
	}

	header = header[len("bytes="):]

	parts := strings.Split(header, "-")

	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid Range format")
	}

	start, err = strconv.ParseInt(parts[0], 10, 64)

	if err != nil {
		return 0, 0, fmt.Errorf("invalid start value")
	}

	if parts[1] != "" {

		end, err = strconv.ParseInt(parts[1], 10, 64)

		if err != nil {
			return 0, 0, fmt.Errorf("invalid end value")
		}
	} else {
		end = -1 // unspecified, will handle later
	}

	return start, end, nil
}
