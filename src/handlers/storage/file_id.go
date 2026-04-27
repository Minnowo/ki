package storage

import (
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidFileID = errors.New("invalid file ID")

const FileIDSize int = 16

type FileID [FileIDSize]byte

func (f *FileID) FromHex(hx string) error {

	n, err := hex.Decode(f[:], []byte(hx))

	if err != nil {
		return err
	}

	if n != FileIDSize {
		return ErrInvalidFileID
	}

	return nil
}

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
