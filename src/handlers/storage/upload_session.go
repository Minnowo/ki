package storage

import (
	"fmt"
	"hash"
	"io"
	"os"
	"sync"
	"time"
)

var ErrSessionNotFound = fmt.Errorf("upload session not found")

// UploadSession holds all in-progress state for a chunked upload.
// The AES-CTR cipher writer and hash writers maintain their state between
// sequential chunk writes, so no post-processing is required at completion.
type UploadSession struct {
	mu sync.Mutex

	// Remains open until the session is finished or deleted.
	TempFile *os.File

	CipherWriter io.Writer
	SHA512H      hash.Hash
	SHA256H      hash.Hash
	SHA1H        hash.Hash
	MD5H         hash.Hash

	AESKey           []byte
	Filename         string
	ExpiresIn        time.Duration
	AllowedDownloads int
	MemoryOnly       bool
	HasPassword      bool
	PasswordHash     []byte
	TotalBytes       int64
	CreatedAt        time.Time
	Username         string

	// LastActivity can be read without holding mu.
	LastActivity activityClock

	// closed is set once TempFile is closed. A request which got this session from the store
	// before it was closed must check this after taking mu.
	closed bool
}

func (s *UploadSession) Write(b []byte) (int, error) {

	// hash writes should never fail
	s.SHA512H.Write(b)
	s.SHA256H.Write(b)
	s.SHA1H.Write(b)
	s.MD5H.Write(b)

	return s.CipherWriter.Write(b)
}

// Cleanup closes and deletes the temp file. The caller must hold mu.
func (s *UploadSession) Cleanup() {

	if s.closed {
		return
	}

	s.closed = true
	s.TempFile.Close()
	os.Remove(s.TempFile.Name())
}
