package storage

import (
	"crypto/rand"
	"io"
	"ki/src/config"
	"ki/src/handlers/crypto"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestSafeFileDownloadExpirey(t *testing.T) {
	assert := assert.New(t)

	tempDir := t.TempDir()

	for _, password := range []string{"", "hello world"} {

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		assert.Nil(err, "bcrypt GenerateFromPassword should not fail")

		tempFile, err := os.CreateTemp(tempDir, "*")
		assert.Nil(err, "should be able to create a temp file")

		var garbage [256]byte
		rand.Read(garbage[:])
		tempFile.Write(garbage[:])
		tempFile.Close()

		file := KiFile{
			Key:              make([]byte, crypto.AES256),
			FilePath:         tempFile.Name(),
			UserPasswordHash: hash,
			KeySize:          config.AES_KEY_SIZE,
			KiMetadata: KiMetadata{
				Downloads:        0,
				AllowedDownloads: 1,
				ActiveDownloads:  0,
				UserSetPassword:  len(password) > 0,
				Expires:          time.Now().Add(24 * time.Hour),
			}}

		// has 1 download remaining
		assert.False(file.IsExpired(), "expired before downloading anything")

		// Simulate a download of the file
		file.Metadata().AddDownloader()
		assert.Equal(file.Metadata().ActiveDownloads, 1, "expected 1 active download")

		// should be expired now
		assert.True(file.IsExpired(), "expected to be expired")

		// We should still be able to get a download of this file though.
		reader, err := file.NewReader(password)
		assert.Nil(err, "should be able to get a reader for this file")

		_, err = io.Copy(io.Discard, reader)
		assert.Nil(err)
		reader.Close()

		// shouldn't be cleaned yet, since ActiveDownloads > 0
		assert.False(file.Clean(), "shouldn't be clean because still downloading")
		assert.False(file.WasCleaned, "shouldn't be clean because still downloading")

		file.Metadata().SubDownloader()

		assert.Equal(file.Metadata().ActiveDownloads, 0, "expected 0 active download")

		assert.True(file.Clean(), "should be able to clean now")
	}
}
