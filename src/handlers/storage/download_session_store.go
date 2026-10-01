package storage

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DownloadSessionStore is an in-memory store for active chunked download sessions.
type DownloadSessionStore struct {
	mu       sync.RWMutex
	sessions map[SessionToken]*DownloadSession
	timeout  time.Duration
}

func newDownloadSessionStore(timeout time.Duration) *DownloadSessionStore {
	return &DownloadSessionStore{
		sessions: make(map[SessionToken]*DownloadSession),
		timeout:  timeout,
	}
}

func (s *DownloadSessionStore) add(id SessionToken, session *DownloadSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = session
}

func (s *DownloadSessionStore) get(id SessionToken) (*DownloadSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *DownloadSessionStore) remove(id SessionToken) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// idle returns the sessions which have been idle for longer than the timeout.
func (s *DownloadSessionStore) idle() map[SessionToken]*DownloadSession {

	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	idle := make(map[SessionToken]*DownloadSession)

	for id, session := range s.sessions {
		if session.LastActivity.IdleFor(now, s.timeout) {
			idle[id] = session
		}
	}

	return idle
}

// removeExpired closes and removes timed-out sessions from the store.
//
// Each idle session is locked before being closed, so a session is never closed while a request is using it.
// The store lock is not held while waiting on a session, since a request holding the session lock may need the store lock to remove it.
func (s *DownloadSessionStore) removeExpired() {

	for id, session := range s.idle() {

		session.mu.Lock()

		// a request may have used the session while we were waiting for the lock
		if !session.closed && session.LastActivity.IdleFor(time.Now(), s.timeout) {

			session.Close()

			s.remove(id)

			log.Info().Str("id", id.Hex()).Msg("swept expired download session")
		}

		session.mu.Unlock()
	}
}
