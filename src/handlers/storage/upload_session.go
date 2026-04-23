package storage

import (
	"fmt"
	"hash"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var ErrSessionNotFound = fmt.Errorf("upload session not found")
var ErrChunkUploadTruncatedWrite = fmt.Errorf("truncated write: wrote less data than we read")

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
	LastActivity     time.Time
	Username         string
}

func (s *UploadSession) Write(b []byte) (int, error) {

	// hash writes should never fail
	s.SHA512H.Write(b)
	s.SHA256H.Write(b)
	s.SHA1H.Write(b)
	s.MD5H.Write(b)

	return s.CipherWriter.Write(b)
}

type UploadSessionStore struct {
	mu       sync.RWMutex
	sessions map[FileID]*UploadSession
	timeout  time.Duration
}

func newUploadSessionStore(timeout time.Duration) *UploadSessionStore {
	return &UploadSessionStore{
		sessions: make(map[FileID]*UploadSession),
		timeout:  timeout,
	}
}

func (s *UploadSessionStore) add(ID FileID, session *UploadSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[ID] = session
}

func (s *UploadSessionStore) get(ID FileID) (*UploadSession, bool) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[ID]

	return session, ok
}

func (s *UploadSessionStore) remove(ID FileID) {

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, ID)
}

func (s *UploadSessionStore) ClearExpired() {

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for id, session := range s.sessions {

		if now.Sub(session.LastActivity) > s.timeout {

			session.TempFile.Close()
			os.Remove(session.TempFile.Name())

			delete(s.sessions, id)

			log.Info().Str("id", id.Hex()).Msg("swept expired upload session")
		}
	}
}
