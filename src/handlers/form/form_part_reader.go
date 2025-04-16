package form

import (
	"mime/multipart"
	"strconv"
)

func ReadFormInt(part *multipart.Part) (int32, bool) {

	// 10 digits is the max length of a int32
	const INT_MAX_CHAR_LEN int = 10

	// read 1 extra, if we get it, the sender is sending invalid data
	var buf [INT_MAX_CHAR_LEN + 1]byte

	n, _ := part.Read(buf[:])

	if 0 < n && n <= INT_MAX_CHAR_LEN {

		val, err := strconv.Atoi(string(buf[:n]))

		if err == nil {
			return int32(val), true
		}
	}

	return 0, false
}
