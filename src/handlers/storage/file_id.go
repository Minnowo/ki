package storage

import (
	"encoding/hex"
	"ki/src/config"
	"strings"
)

type FileID [config.FILE_ID_SIZE]byte

func (f FileID) Hex() string {
	return hex.EncodeToString(f[:])
}

func (f FileID) ToPretty() string {

	var sb strings.Builder
	sb.Grow(len(f)*2 + len(f)/2)

	for i := 0; i < len(f); i += 2 {
		sb.WriteString(hex.EncodeToString(f[i : i+2]))
		sb.WriteByte(' ')
	}
	return sb.String()
}
