package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type closeTracker struct {
	*strings.Reader
	Closed bool
}

func (c *closeTracker) Close() error {
	c.Closed = true
	return nil
}

func TestDownloadSessionClose(t *testing.T) {

	stream := &closeTracker{Reader: strings.NewReader("data")}
	session := &DownloadSession{Stream: stream}

	assert.Nil(t, session.Close(), "close should not error")
	assert.True(t, stream.Closed, "close should close the underlying stream")
}

func TestDownloadSessionStoreRemoveExpired(t *testing.T) {

	store := newDownloadSessionStore(time.Minute)

	var expiredID, activeID SessionToken
	expiredID.New()
	activeID.New()

	expiredStream := &closeTracker{Reader: strings.NewReader("data")}
	activeStream := &closeTracker{Reader: strings.NewReader("data")}

	store.add(expiredID, &DownloadSession{Stream: expiredStream, LastActivity: time.Now().Add(-time.Hour)})
	store.add(activeID, &DownloadSession{Stream: activeStream, LastActivity: time.Now()})

	store.removeExpired()

	_, ok := store.get(expiredID)
	assert.False(t, ok, "expired session should be removed")
	assert.True(t, expiredStream.Closed, "expired session stream should be closed")

	_, ok = store.get(activeID)
	assert.True(t, ok, "active session should remain")
	assert.False(t, activeStream.Closed, "active session stream should stay open")
}
