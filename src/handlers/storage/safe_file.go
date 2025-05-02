package storage

import (
	"crypto/rand"
	"fmt"
	"io"
	"ki/src/config"
	"ki/src/handlers/crypto"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	ErrInvalidUpload error = fmt.Errorf("upload is invalid")
)

type SafeFileUpload struct {
	ExpiresIn        time.Duration
	AllowedDownloads int
	Filename         string
	Password         string
}

func (f *SafeFileUpload) Valid() error {

	if f.ExpiresIn.Milliseconds() < time.Minute.Milliseconds() {
		return fmt.Errorf("%w: expirey time must be at least 1 minute", ErrInvalidUpload)
	}

	if f.AllowedDownloads <= 0 {
		return fmt.Errorf("%w: must have at least 1 download", ErrFileExpired)
	}

	if len(f.Password) > config.MAX_PASSWORD_LENGTH {
		return fmt.Errorf("%w: password length must be less than %d", ErrFileExpired, config.MAX_PASSWORD_LENGTH)
	}

	return nil
}

type SafeFile struct {
	Sha512Hash       []byte
	Sha256Hash       []byte
	Sha1Hash         []byte
	Md5Hash          []byte
	Size             int64
	Expires          time.Time
	AllowedDownloads int
	Downloads        int
	Name             string
	UserSetPassword  bool
}

func (f *SafeFile) IsExpired() bool {
	return time.Now().After(f.Expires) || f.Downloads >= f.AllowedDownloads
}

// deep copies this file
func (f *SafeFile) Copy() *SafeFile {
	var sf SafeFile
	sf.Sha512Hash = make([]byte, len(f.Sha512Hash))
	copy(sf.Sha512Hash, f.Sha512Hash)
	sf.Sha256Hash = make([]byte, len(f.Sha256Hash))
	copy(sf.Sha256Hash, f.Sha256Hash)
	sf.Sha1Hash = make([]byte, len(f.Sha1Hash))
	copy(sf.Sha1Hash, f.Sha1Hash)
	sf.Md5Hash = make([]byte, len(f.Md5Hash))
	copy(sf.Md5Hash, f.Md5Hash)
	sf.Size = f.Size
	sf.Expires = f.Expires
	sf.AllowedDownloads = f.AllowedDownloads
	sf.Downloads = f.Downloads
	sf.Name = f.Name
	sf.UserSetPassword = f.UserSetPassword
	return &sf
}

type SafeFileEx struct {
	SafeFile
	sync.RWMutex

	// the path to the encrypted file
	filePath string

	// the key used for encrypting/decrypting the file
	key []byte

	// if the user set a password, this is a bcrypt hash of the password.
	// in such a case the key above is the key to a one-time-pad.
	// when combined with the users password it produces the key used for encrypting/decrypting the file
	userPasswordHash []byte

	// set true if the file was deleted from disk
	wasCleaned      bool
	activeDownloads int
}

func (f *SafeFileEx) IsExpiredSync() bool {
	f.RLock()
	defer f.RUnlock()
	return f.IsExpired()
}

type DownloadData struct {
	Reader io.Reader
	File   *os.File
	Name   string
}

// if true a new download has been counted.
// returns false if the file is expired.
func (f *SafeFileEx) StartDownload(kSize crypto.AESKeySize, password string) (*DownloadData, error) {

	f.Lock()
	defer f.Unlock()

	if f.IsExpired() || f.Cleaned() {
		return nil, ErrFileExpired
	}

	fileHandle, err := os.Open(f.filePath)

	if err != nil {
		return nil, err
	}

	var aesr io.Reader

	if f.UserSetPassword {
		aesr, err = crypto.GetStreamDecryptionReaderEx(kSize, f.key, password, fileHandle)
	} else {
		aesr, err = crypto.GetStreamDecryptionReader(f.key, fileHandle)
	}

	if err != nil {
		fileHandle.Close()
		return nil, err
	}

	f.activeDownloads += 1
	f.Downloads += 1

	dl := &DownloadData{
		File:   fileHandle,
		Reader: aesr,
		Name:   f.Name,
	}

	log.Debug().Msg("starting download")
	return dl, nil
}

func (f *SafeFileEx) StopDownload() {
	f.Lock()
	defer f.Unlock()
	if f.activeDownloads > 0 {
		f.activeDownloads -= 1
	} else {
		log.Error().Msg("stopping download but there wasn't any running download")
	}
	log.Debug().Msg("stopping download")
}

func (f *SafeFileEx) Cleaned() bool {
	return f.wasCleaned
}

// if the file was deleted does nothing
// otherwise expires the file and if there are no active downloads deletes the file.
// returns true if cleaned.
func (f *SafeFileEx) Clean() bool {

	f.Lock()
	defer f.Unlock()

	if f.Cleaned() {
		return true
	}

	log.Debug().Str("name", f.Name).Msg("expiring file")

	f.Expires = time.Unix(0, 0)

	if f.key != nil {
		rand.Read(f.key)
	}

	if f.activeDownloads > 0 {
		return false
	}

	// we want to keep trying to clean until the file is removed
	if err := os.Remove(f.filePath); err != nil {

		if os.IsNotExist(err) {
			f.wasCleaned = true
		} else {
			log.Warn().Err(err).Msg("failed to remove file")
		}

	} else {
		f.wasCleaned = true
	}

	if f.wasCleaned {
		log.Info().Str("path", f.filePath).Msg("file was cleaned")
		return true
	}
	return false
}

// checks if the file is expired and calls clean if it is
// returns true if expired otherwise false
func (f *SafeFileEx) CleanIfExpired() bool {

	f.RLock()
	isExpired := f.IsExpired()
	wasCleaned := f.Cleaned()
	f.RUnlock()

	if !isExpired {
		return false
	}

	if !wasCleaned {
		f.Clean()
	}

	return true
}
