package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

var ErrInvalidSessionID = errors.New("invalid session ID")

const SessionTokenSize = 16

type SessionToken [SessionTokenSize]byte

func (a *SessionToken) New() {
	rand.Read(a[:])
}

func (f *SessionToken) FromHex(hx string) error {

	if len(hx) != 2*SessionTokenSize {
		return ErrInvalidSessionID
	}

	var tmp SessionToken

	if n, err := hex.Decode(tmp[:], []byte(hx)); err != nil {
		return err
	} else if n != SessionTokenSize {
		return ErrInvalidSessionID
	}

	copy(f[:], tmp[:])

	return nil
}

func (f SessionToken) Hex() string {
	return hex.EncodeToString(f[:])
}
