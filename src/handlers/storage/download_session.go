package storage

import (
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DownloadSession holds an open decryption reader for a chunked download.
// The AES-CTR stream state is maintained naturally as the reader is consumed
// sequentially across multiple chunk requests.
type DownloadSession struct {
	mu sync.Mutex

	// Open decryption reader; stays alive between chunk requests.
	Stream io.ReadCloser

	// Total file size, reported to the client so it knows when all chunks are received.
	TotalBytes int64

	// Bytes delivered to the client so far.
	BytesRead int64

	// FileID of the file being downloaded, needed to call SubDownloader on cleanup.
	FileID FileID

	LastActivity time.Time

	// buf is a fixed size buffer containing the last read data from the stream.
	buf []byte

	// pending, if len() > 0, is a slice of buf containing the next data to read.
	// When pending is nil or has len() == 0, buf should be updated with new data and pending should be updated.
	pending []byte
}

func (d *DownloadSession) Read(p []byte) (int, error) {

	n := int(0)

	for {

		for len(d.pending) > 0 {

			m := copy(p, d.pending)
			n += m

			p = p[m:]
			d.pending = d.pending[m:]

			if len(p) == 0 {
				return n, nil
			}
		}

		m, err := d.Stream.Read(d.buf)

		d.pending = d.buf[0:m]

		if m > 0 {
			continue
		}

		if err == nil {
			err = io.EOF
		}

		return n, err
	}
}

// DownloadSessionStore is an in-memory store for active chunked download sessions.
type DownloadSessionStore struct {
	mu       sync.RWMutex
	sessions map[FileID]*DownloadSession
	timeout  time.Duration
}

func newDownloadSessionStore(timeout time.Duration) *DownloadSessionStore {
	return &DownloadSessionStore{
		sessions: make(map[FileID]*DownloadSession),
		timeout:  timeout,
	}
}

func (s *DownloadSessionStore) add(id FileID, session *DownloadSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = session
}

func (s *DownloadSessionStore) get(id FileID) (*DownloadSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

func (s *DownloadSessionStore) remove(id FileID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

// removeExpired removes timed-out sessions from the store and returns them.
// The caller is responsible for closing each session's Stream and calling SubDownloader.
func (s *DownloadSessionStore) removeExpired() []*DownloadSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var expired []*DownloadSession

	for id, session := range s.sessions {
		if now.Sub(session.LastActivity) > s.timeout {
			delete(s.sessions, id)
			expired = append(expired, session)
			log.Info().Str("id", id.Hex()).Msg("swept expired download session")
		}
	}

	return expired
}
