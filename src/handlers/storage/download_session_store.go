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

// removeExpired closes removes timed-out sessions from the store.
func (s *DownloadSessionStore) removeExpired() {

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for id, session := range s.sessions {

		if now.Sub(session.LastActivity) > s.timeout {

			session.Close()

			delete(s.sessions, id)

			log.Info().Str("id", id.Hex()).Msg("swept expired download session")
		}
	}
}
