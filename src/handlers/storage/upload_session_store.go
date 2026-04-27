package storage

import (
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type UploadSessionStore struct {
	mu       sync.RWMutex
	sessions map[SessionToken]*UploadSession
	timeout  time.Duration
}

func newUploadSessionStore(timeout time.Duration) *UploadSessionStore {
	return &UploadSessionStore{
		sessions: make(map[SessionToken]*UploadSession),
		timeout:  timeout,
	}
}

func (s *UploadSessionStore) add(ID SessionToken, session *UploadSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[ID] = session
}

func (s *UploadSessionStore) get(ID SessionToken) (*UploadSession, bool) {

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[ID]

	return session, ok
}

func (s *UploadSessionStore) remove(ID SessionToken) {

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
