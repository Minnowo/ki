package storage

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func newTestHandlerWithFile(t *testing.T, store FileStore, password string, allowedDownloads int) (StorageHandler, FileID) {
	t.Helper()

	f := NewStorageHandler(t.TempDir(), store, bcrypt.MinCost)

	id, err := f.SaveFile(FileUpload{
		FStream:          strings.NewReader("file contents"),
		ExpiresIn:        time.Hour,
		AllowedDownloads: allowedDownloads,
		Filename:         "test.txt",
		Password:         password,
	})
	assert.NoError(t, err)

	return f, *id
}

func TestReadFile(t *testing.T) {

	for _, store := range getStores(t) {

		t.Run("wrong password does not count a download", func(t *testing.T) {
			f, id := newTestHandlerWithFile(t, store, "secret", 1)

			err := f.ReadFile(io.Discard, id, "wrong")
			assert.ErrorIs(t, err, ErrNeedsAuth)

			file, ok := store.GetFile(id)
			assert.True(t, ok)
			assert.Equal(t, 0, file.Downloads)
		})

		t.Run("correct password reads the file and counts a download", func(t *testing.T) {
			f, id := newTestHandlerWithFile(t, store, "secret", 2)

			var out bytes.Buffer
			err := f.ReadFile(&out, id, "secret")
			assert.NoError(t, err)
			assert.Equal(t, "file contents", out.String())

			file, ok := store.GetFile(id)
			assert.True(t, ok)
			assert.Equal(t, 1, file.Downloads)
		})

		t.Run("no downloads after the limit", func(t *testing.T) {
			f, id := newTestHandlerWithFile(t, store, "", 1)

			assert.NoError(t, f.ReadFile(io.Discard, id, ""))
			assert.ErrorIs(t, f.ReadFile(io.Discard, id, ""), ErrFileExpired)
		})

		t.Run("non-existent file", func(t *testing.T) {
			f := NewStorageHandler(t.TempDir(), store, bcrypt.MinCost)

			assert.ErrorIs(t, f.ReadFile(io.Discard, nonExistID, ""), ErrFileNotFound)
		})
	}
}

// The password is checked before the download is counted, so many concurrent downloads
// can all pass the check. Only as many as are allowed should actually get the file.
func TestReadFile_ConcurrentDownloadsRespectLimit(t *testing.T) {

	for _, store := range getStores(t) {

		const allowed = 3
		const attempts = 20

		f, id := newTestHandlerWithFile(t, store, "secret", allowed)

		var wg sync.WaitGroup
		var succeeded atomic.Int32

		for range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()

				if f.ReadFile(io.Discard, id, "secret") == nil {
					succeeded.Add(1)
				}
			}()
		}

		wg.Wait()

		assert.Equal(t, int32(allowed), succeeded.Load())
	}
}

func TestBeginChunkedDownload(t *testing.T) {

	for _, store := range getStores(t) {

		t.Run("wrong password does not count a download", func(t *testing.T) {
			f, id := newTestHandlerWithFile(t, store, "secret", 1)

			_, _, err := f.BeginChunkedDownload(id, "wrong")
			assert.ErrorIs(t, err, ErrNeedsAuth)

			file, ok := store.GetFile(id)
			assert.True(t, ok)
			assert.Equal(t, 0, file.Downloads)
		})

		t.Run("correct password opens a session", func(t *testing.T) {
			f, id := newTestHandlerWithFile(t, store, "secret", 1)

			sessionID, meta, err := f.BeginChunkedDownload(id, "secret")
			assert.NoError(t, err)
			assert.Equal(t, int64(len("file contents")), meta.Size)

			var out bytes.Buffer
			err = f.WithDownloadSession(sessionID, id, func(session *DownloadSession) error {
				_, err := session.WriteToN(&out, session.TotalBytes)
				return err
			})
			assert.NoError(t, err)
			assert.Equal(t, "file contents", out.String())
		})
	}
}
