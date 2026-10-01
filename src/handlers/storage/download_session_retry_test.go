package storage

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// retryTestContent is 100 bytes where every byte is different from its neighbours,
// so reading from the wrong offset can't give the right data.
func retryTestContent() []byte {
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}
	return content
}

// beginRetryTestDownload uploads retryTestContent and starts a chunked download of it.
func beginRetryTestDownload(t *testing.T) (*StorageHandler, FileID, SessionToken, []byte) {
	t.Helper()

	content := retryTestContent()

	f := NewStorageHandler(t.TempDir(), NewMemoryFileStore(), bcrypt.MinCost)

	id, err := f.SaveFile(FileUpload{
		FStream:          bytes.NewReader(content),
		ExpiresIn:        time.Hour,
		AllowedDownloads: 1,
		Filename:         "test.bin",
	})
	assert.NoError(t, err)

	sessionID, _, err := f.BeginChunkedDownload(*id, "")
	assert.NoError(t, err)

	return &f, *id, sessionID, content
}

// readChunk does what the chunk download handler does for a request starting at start.
func readChunk(f *StorageHandler, sessionID SessionToken, id FileID, start int64, length int64) ([]byte, error) {

	var out bytes.Buffer

	err := f.WithDownloadSession(sessionID, id, func(session *DownloadSession) error {

		if err := session.StartChunk(start); err != nil {
			return err
		}

		_, err := session.WriteToN(&out, length)

		return err
	})

	return out.Bytes(), err
}

func TestDownloadSessionRetry(t *testing.T) {

	t.Run("retry the last chunk", func(t *testing.T) {
		f, id, sid, content := beginRetryTestDownload(t)

		chunk, err := readChunk(f, sid, id, 0, 16)
		assert.NoError(t, err)
		assert.Equal(t, content[0:16], chunk)

		// the response for this chunk is lost on the way to the client
		_, err = readChunk(f, sid, id, 16, 16)
		assert.NoError(t, err)

		chunk, err = readChunk(f, sid, id, 16, 16)
		assert.NoError(t, err, "retrying the last chunk should be allowed")
		assert.Equal(t, content[16:32], chunk, "retry should resend the same bytes")

		chunk, err = readChunk(f, sid, id, 32, 68)
		assert.NoError(t, err)
		assert.Equal(t, content[32:100], chunk, "the download should carry on after a retry")
	})

	t.Run("retry the rest of a partly received chunk", func(t *testing.T) {
		f, id, sid, content := beginRetryTestDownload(t)

		_, err := readChunk(f, sid, id, 0, 16)
		assert.NoError(t, err)

		// the client got the first 4 bytes of this chunk before the connection dropped
		_, err = readChunk(f, sid, id, 16, 16)
		assert.NoError(t, err)

		chunk, err := readChunk(f, sid, id, 20, 12)
		assert.NoError(t, err)
		assert.Equal(t, content[20:32], chunk)
	})

	t.Run("retry the same chunk more than once", func(t *testing.T) {
		f, id, sid, content := beginRetryTestDownload(t)

		for range 3 {
			chunk, err := readChunk(f, sid, id, 0, 16)
			assert.NoError(t, err)
			assert.Equal(t, content[0:16], chunk)
		}
	})

	t.Run("retry the final chunk", func(t *testing.T) {
		f, id, sid, content := beginRetryTestDownload(t)

		_, err := readChunk(f, sid, id, 0, 90)
		assert.NoError(t, err)

		// the final chunk is lost, the session must stay open so it can be retried
		_, err = readChunk(f, sid, id, 90, 10)
		assert.NoError(t, err)

		chunk, err := readChunk(f, sid, id, 90, 10)
		assert.NoError(t, err, "the final chunk should be retryable until the client says it's done")
		assert.Equal(t, content[90:100], chunk)
	})

	t.Run("cannot rewind before the last chunk", func(t *testing.T) {
		f, id, sid, content := beginRetryTestDownload(t)

		_, err := readChunk(f, sid, id, 0, 16)
		assert.NoError(t, err)

		_, err = readChunk(f, sid, id, 16, 16)
		assert.NoError(t, err)

		_, err = readChunk(f, sid, id, 0, 16)
		assert.ErrorIs(t, err, ErrInvalidSeek, "rewinding past the last chunk would allow reading the file more than once")

		_, err = readChunk(f, sid, id, 15, 1)
		assert.ErrorIs(t, err, ErrInvalidSeek, "the last chunk started at 16")

		chunk, err := readChunk(f, sid, id, 32, 16)
		assert.NoError(t, err, "a refused rewind should not break the session")
		assert.Equal(t, content[32:48], chunk)
	})

	t.Run("cannot read the whole file twice", func(t *testing.T) {
		f, id, sid, _ := beginRetryTestDownload(t)

		_, err := readChunk(f, sid, id, 0, 50)
		assert.NoError(t, err)

		_, err = readChunk(f, sid, id, 50, 50)
		assert.NoError(t, err)

		_, err = readChunk(f, sid, id, 0, 100)
		assert.ErrorIs(t, err, ErrInvalidSeek)
	})

	t.Run("skipping ahead is still allowed", func(t *testing.T) {
		f, id, sid, content := beginRetryTestDownload(t)

		_, err := readChunk(f, sid, id, 0, 16)
		assert.NoError(t, err)

		chunk, err := readChunk(f, sid, id, 48, 16)
		assert.NoError(t, err)
		assert.Equal(t, content[48:64], chunk)
	})

	t.Run("done closes the session", func(t *testing.T) {
		f, id, sid, _ := beginRetryTestDownload(t)

		_, err := readChunk(f, sid, id, 0, 100)
		assert.NoError(t, err)

		assert.NoError(t, f.AbortDownloadSession(sid, id))

		_, err = readChunk(f, sid, id, 0, 100)
		assert.ErrorIs(t, err, ErrSessionNotFound)
	})
}
