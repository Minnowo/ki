package storage

import (
	"fmt"
	"io"
	"ki/src/config"
	"time"
)

var (
	ErrInvalidUpload error = fmt.Errorf("upload is invalid")
)

type FileUpload struct {
	FStream          io.Reader
	ExpiresIn        time.Duration
	AllowedDownloads int
	MemoryOnly       bool
	Filename         string
	Password         string
}

func (f *FileUpload) Valid() error {

	if f.ExpiresIn.Milliseconds() < time.Minute.Milliseconds() {
		return fmt.Errorf("%w: expirey time must be at least 1 minute", ErrInvalidUpload)
	}

	if f.AllowedDownloads <= 0 {
		return fmt.Errorf("%w: must have at least 1 download", ErrFileExpired)
	}

	if len(f.Password) > config.MAX_PASSWORD_LENGTH {
		return fmt.Errorf("%w: password length must be less than %d", ErrFileExpired, config.MAX_PASSWORD_LENGTH)
	}

	return nil
}

func (f *FileUpload) Read(p []byte) (n int, err error) {
	return f.FStream.Read(p)
}
