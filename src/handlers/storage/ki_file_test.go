package storage

import (
	"crypto/rand"
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
				Size:             int64(len(garbage)),
				Downloads:        0,
				AllowedDownloads: 1,
				UserSetPassword:  len(password) > 0,
				Expires:          time.Now().Add(24 * time.Hour),
			}}

		// has 1 download remaining
		assert.False(file.IsExpired(), "expired before downloading anything")

		// count the download
		file.Metadata().CountDownload()

		// should be expired now
		assert.True(file.IsExpired(), "expected to be expired")

		// We should still be able to get a download of this file though.
		reader, err := file.NewReader(password)
		assert.Nil(err, "should be able to get a reader for this file")
		reader.Close()

		assert.True(file.Clean(), "should be able to clean now")
	}
}
