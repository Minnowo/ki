package storage

import (
	"os"
	"testing"
	"time"
)

func TestSafeFileDownloadExpirey(t *testing.T) {

	tempDir := t.TempDir()
	tempFile, err := os.CreateTemp(tempDir, "*")

	if err != nil {
		t.Error("could not create temp file")
		t.FailNow()
		return
	}
	defer tempFile.Close()

	file := SafeFileEx{
		filePath:        tempFile.Name(),
		activeDownloads: 0,
		SafeFile: SafeFile{
			Downloads:        0,
			AllowedDownloads: 1,
			Expires:          time.Now().Add(24 * time.Hour),
		}}

	// has 1 download remaining
	if file.IsExpired() {
		t.Error("expired before downloading anything")
		t.Fail()
		return
	}

	// should use the download
	file.StartDownload()

	if file.activeDownloads != 1 {
		t.Error("expected 1 active download")
		t.Fail()
		return
	}

	// should be expired now
	if !file.IsExpired() || !file.CleanIfExpired() {
		t.Error("expected to be expired")
		t.Fail()
		return
	}

	// shouldn't be cleaned yet
	if file.Clean() || file.wasCleaned {
		t.Error("shouldn't be clean because still downloading")
		t.Fail()
		return
	}

	file.StopDownload()

	if file.activeDownloads != 0 {
		t.Error("expected 0 active download")
		t.Fail()
		return
	}

	if !file.Clean() {
		t.Error("should be able to clean now")
		t.Fail()
		return
	}
}
