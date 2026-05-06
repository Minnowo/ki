package storage

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"io"
	"ki/src/config"
	"net/http"
	"os"
	"time"

	"ki/src/handlers/crypto"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNeedsAuth    error = errors.New("needs password to download")
	ErrFileExpired  error = errors.New("file has expired")
	ErrFileNotFound error = errors.New("file does not exist")
)

// StorageHandler handles uploading and downloading files, and keeping them in a file store.
type StorageHandler struct {
	FileDir              string
	bcryptCost           int
	keySize              crypto.AESKeySize
	metadataStore        FileStore
	daemonCloseChan      chan bool
	uploadSessionStore   *UploadSessionStore
	downloadSessionStore *DownloadSessionStore
}

func NewStorageHandler(dir string, store FileStore, bcryptCost int) StorageHandler {
	return StorageHandler{
		FileDir:              dir,
		metadataStore:        store,
		keySize:              config.AES_KEY_SIZE,
		bcryptCost:           bcryptCost,
		daemonCloseChan:      nil,
		uploadSessionStore:   newUploadSessionStore(config.SessionTimeout()),
		downloadSessionStore: newDownloadSessionStore(config.SessionTimeout()),
	}
}

// ShutdownExpireCheckLoop signals the shutdown of the loop which expires files in the store.
func (f *StorageHandler) ShutdownExpireCheckLoop() {

	if f.daemonCloseChan == nil {
		return
	}

	f.daemonCloseChan <- true
	close(f.daemonCloseChan)
	f.daemonCloseChan = nil
}

// RunExpireCheckLoop creates a goroutine which periodically checks and expires files.
// Returns true if a new loop was started, otherwise false.
func (f *StorageHandler) RunExpireCheckLoop(interval time.Duration) bool {

	if f.daemonCloseChan != nil {
		return false
	}

	done := make(chan bool)

	f.daemonCloseChan = done

	log.Debug().Msg("file expire check daemon starting")

	go func() {

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				log.Debug().Msg("file expire check daemon done")
				return

			case t := <-ticker.C:

				f.metadataStore.ClearExpiredFiles()
				f.uploadSessionStore.ClearExpired()
				f.downloadSessionStore.removeExpired()

				log.Debug().Str("time", t.String()).Msg("clearing out expired files")
			}
		}
	}()

	return true
}

func (f *StorageHandler) FileMetadata(key FileID) *KiMetadata {

	v, ok := f.metadataStore.GetFileMetadata(key)

	if !ok {
		return nil
	}

	return v
}

// SaveFile uploads the given file into the file store.
func (f *StorageHandler) SaveFile(upload FileUpload) (*FileID, error) {
	return f.SaveFileWithProgress(upload, nil)
}

// SaveFileWithProgress uploads the given file into the file store calling the update method
// with the number of bytes read as the file is processed.
func (f *StorageHandler) SaveFileWithProgress(upload FileUpload, update func(int)) (*FileID, error) {

	if err := upload.Valid(); err != nil {
		return nil, err
	}

	file, err := os.CreateTemp(f.FileDir, config.FILENAME_PREFIX+"*")

	if err != nil {
		return nil, err
	}

	return func() (id *FileID, err error) {

		defer func() {

			file.Close()

			if err != nil {
				os.Remove(file.Name())
			}
		}()

		id, err = f.saveFileWithProgress(file, upload, update)

		return
	}()
}

func (f *StorageHandler) saveFileWithProgress(file *os.File, upload FileUpload, update func(int)) (*FileID, error) {

	didUserGivePassword := upload.Password != ""

	key := make([]byte, f.keySize)
	rand.Read(key)

	var err error
	var aesw io.Writer

	if didUserGivePassword {
		aesw, err = crypto.GetStreamEncryptionWriterEx(f.keySize, key, upload.Password, file)
	} else {
		aesw, err = crypto.GetStreamEncryptionWriter(key, file)
	}

	if err != nil {
		return nil, err
	}

	var passHash []byte = nil

	if didUserGivePassword {

		passHash, err = bcrypt.GenerateFromPassword([]byte(upload.Password), f.bcryptCost)

		if err != nil {
			return nil, err
		}
	}

	sha512Hash := sha512.New()
	sha256Hash := sha256.New()
	sha1Hash := sha1.New()
	md5Hash := md5.New()

	fileSize := int64(0)
	buffer := make([]byte, 4*config.KB)
	for {

		n, err := upload.Read(buffer)

		if n > 0 {

			sha512Hash.Write(buffer[0:n])
			sha256Hash.Write(buffer[0:n])
			sha1Hash.Write(buffer[0:n])
			md5Hash.Write(buffer[0:n])

			wN, wErr := aesw.Write(buffer[0:n])

			fileSize += int64(wN)

			if update != nil {
				update(wN)
			}

			if wErr != nil {
				log.Error().Err(wErr).Msg("error writing data to disk")
				return nil, wErr
			}

			if n != wN {
				return nil, ErrTruncatedWrite
			}

			if fileSize > config.MaxUploadSize() {

				log.Info().Msg("max upload size exceeded")

				return nil, ErrMaxUploadSizeExceeded
			}
		}

		if err != nil {

			if err == io.EOF {
				break
			}

			if _, ok := err.(*http.MaxBytesError); ok {
				return nil, ErrMaxUploadSizeExceeded
			}

			return nil, err
		}

		if n == 0 {
			break
		}
	}

	fileId, err := f.metadataStore.StoreFile(&KiFile{
		UserPasswordHash: passHash,
		Key:              key,
		FilePath:         file.Name(),
		KeySize:          f.keySize,
		KiMetadata: KiMetadata{
			Sha512Hash:       sha512Hash.Sum(nil),
			Sha256Hash:       sha256Hash.Sum(nil),
			Sha1Hash:         sha1Hash.Sum(nil),
			Md5Hash:          md5Hash.Sum(nil),
			Expires:          time.Now().Add(upload.ExpiresIn),
			Size:             fileSize,
			Name:             upload.Filename,
			Downloads:        0,
			AllowedDownloads: upload.AllowedDownloads,
			UserSetPassword:  didUserGivePassword,
			MemoryOnly:       upload.MemoryOnly,
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Error storing file")
		return nil, err
	}

	log.Info().
		Str("name", file.Name()).
		Str("expires", upload.ExpiresIn.String()).
		Int("allowedDownloads", upload.AllowedDownloads).
		Int64("size", fileSize).
		Msg("saved new file")

	return &fileId, nil
}
