package storage

import (
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

// idle returns the sessions which have been idle for longer than the timeout.
func (s *UploadSessionStore) idle() map[SessionToken]*UploadSession {

	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	idle := make(map[SessionToken]*UploadSession)

	for id, session := range s.sessions {
		if session.LastActivity.IdleFor(now, s.timeout) {
			idle[id] = session
		}
	}

	return idle
}

// ClearExpired deletes timed-out sessions and their temp files.
//
// Each idle session is locked before being cleaned up, so a session is never cleaned up while a request is using it.
// The store lock is not held while waiting on a session, since a request holding the session lock may need the store lock to remove it.
func (s *UploadSessionStore) ClearExpired() {

	for id, session := range s.idle() {

		session.mu.Lock()

		// a request may have used the session while we were waiting for the lock
		if !session.closed && session.LastActivity.IdleFor(time.Now(), s.timeout) {

			session.Cleanup()

			s.remove(id)

			log.Info().Str("id", id.Hex()).Msg("swept expired upload session")
		}

		session.mu.Unlock()
	}
}
