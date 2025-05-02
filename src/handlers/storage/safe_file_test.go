package storage

import (
	"io"
	"ki/src/handlers/crypto"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSafeFileDownloadExpirey(t *testing.T) {
	assert := assert.New(t)

	tempDir := t.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "*")

	assert.Nil(err)
	defer tempFile.Close()

	file := SafeFileEx{
		key:             make([]byte, crypto.AES256),
		filePath:        tempFile.Name(),
		activeDownloads: 0,
		SafeFile: SafeFile{
			Downloads:        0,
			AllowedDownloads: 1,
			Expires:          time.Now().Add(24 * time.Hour),
		}}

	// has 1 download remaining
	assert.False(file.IsExpired(), "expired before downloading anything")

	// should use the download
	dl, err := file.StartDownload(crypto.AES256, "")
	assert.Nil(err)

	_, err = io.Copy(io.Discard, dl.Reader)
	assert.Nil(err)

	assert.Equal(file.activeDownloads, 1, "expected 1 active download")

	// should be expired now
	assert.True(file.IsExpired(), "expected to be expired")
	assert.True(file.CleanIfExpired(), "expected to be expired")

	// shouldn't be cleaned yet
	assert.False(file.Clean(), "shouldn't be clean because still downloading")
	assert.False(file.wasCleaned, "shouldn't be clean because still downloading")

	file.StopDownload()

	assert.Equal(file.activeDownloads, 0, "expected 0 active download")

	assert.True(file.Clean(), "should be able to clean now")
}
