package storage

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"io"
	"ki/src/config"
	"net/http"
	"os"
	"strconv"
	"time"

	"ki/src/handlers/crypto"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNeedsAuth    error = fmt.Errorf("needs password to download")
	ErrFileExpired  error = fmt.Errorf("file has expired")
	ErrFileNotFound error = fmt.Errorf("file does not exist")
)

// FileUploadHandler handles uploading and downloading files, and keeping them in a file store.
type FileUploadHandler struct {
	FileDir         string
	bcryptCost      int
	keySize         crypto.AESKeySize
	metadataStore   FileStore
	daemonCloseChan chan bool
}

func NewFileStore(dir string, store FileStore, bcryptCost int) FileUploadHandler {
	return FileUploadHandler{
		FileDir:         dir,
		metadataStore:   store,
		keySize:         config.AES_KEY_SIZE,
		bcryptCost:      bcryptCost,
		daemonCloseChan: nil,
	}
}

// ShutdownExpireCheckLoop signals the shutdown of the loop which expires files in the store.
func (f *FileUploadHandler) ShutdownExpireCheckLoop() {

	if f.daemonCloseChan == nil {
		return
	}

	f.daemonCloseChan <- true
	close(f.daemonCloseChan)
	f.daemonCloseChan = nil
}

// RunExpireCheckLoop creates a goroutine which periodically checks and expires files.
// Returns true if a new loop was started, otherwise false.
func (f *FileUploadHandler) RunExpireCheckLoop(interval time.Duration) bool {

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

				log.Debug().Str("time", t.String()).Msg("clearing out expired files")
			}
		}
	}()

	return true
}

func (f *FileUploadHandler) GetFile(key FileID) *KiMetadata {

	v, ok := f.metadataStore.GetFileMetadata(key)

	if !ok {
		return nil
	}

	return v
}

func (f *FileUploadHandler) ReadFile(w io.Writer, key FileID, password string) error {

	file, err := f.metadataStore.WithFile(key, func(file *KiFile) error {

		if file.IsExpired() {
			return ErrFileExpired
		}

		if file.Metadata().UserSetPassword && !file.PasswordIsValid(password) {
			return ErrNeedsAuth
		}

		file.Metadata().AddDownloader()

		return nil
	})

	// file expired, or the user password was wrong
	if err != nil {
		return err
	}

	// We need to clear the ActiveDownload we put from the above call
	defer f.metadataStore.WithFile(key, func(file *KiFile) error {

		file.Metadata().SubDownloader()

		if file.Metadata().ActiveDownloads < 0 {
			log.Error().Msg("active downloads < 0, this should be impossible!")
		}

		return nil
	})

	fstream, err := file.NewReader(password)

	if err != nil {
		return err
	}

	defer fstream.Close()

	if r, ok := w.(http.ResponseWriter); ok {

		r.Header().Set("Content-Length", strconv.FormatInt(file.Metadata().Size, 10))

		if file.Metadata().Name == "" {
			r.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", key.Hex()))
		} else {
			r.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", file.Metadata().Name))
		}
	}

	n, err := io.Copy(w, fstream)

	log.Debug().Int64("n", n).Msg("wrote bytes to client")

	if err != nil {
		log.Warn().Err(err).Msg("error while sending someone a file")
		return err
	}

	return nil
}

func (f *FileUploadHandler) SaveFile(upload FileUpload) (*FileID, error) {
	return f.SaveFileWithProgress(upload, nil)
}

func (f *FileUploadHandler) SaveFileWithProgress(upload FileUpload, update func(int)) (*FileID, error) {

	if err := upload.Valid(); err != nil {
		return nil, err
	}

	file, err := os.CreateTemp(f.FileDir, config.FILENAME_PREFIX+"*")

	if err == nil {
		defer file.Close()
	} else {
		return nil, err
	}

	didUserGivePassword := upload.Password != ""

	key := make([]byte, f.keySize)
	rand.Read(key)

	var aesw io.Writer

	if didUserGivePassword {
		aesw, err = crypto.GetStreamEncryptionWriterEx(f.keySize, key, upload.Password, file)
	} else {
		aesw, err = crypto.GetStreamEncryptionWriter(key, file)
	}

	if err != nil {
		return nil, err
	}

	sha512Hash := sha512.New()
	sha256Hash := sha256.New()
	sha1Hash := sha1.New()
	md5Hash := md5.New()

	fileSize := int64(0)
	buffer := make([]byte, 4*config.KB) // seems to be the buffer size of the http request
	for {

		n, err := upload.Read(buffer)

		if n > 0 {
			fileSize += int64(n)
			aesw.Write(buffer[0:n])
			sha512Hash.Write(buffer[0:n])
			sha256Hash.Write(buffer[0:n])
			sha1Hash.Write(buffer[0:n])
			md5Hash.Write(buffer[0:n])
			if update != nil {
				update(n)
			}
		}

		if err != nil {

			if err == io.EOF {
				break
			}

			file.Close()
			os.Remove(file.Name())

			log.Info().Err(err).Msg("returning error from SaveFile")
			return nil, err
		}

		if n == 0 {
			break
		}
	}

	var passHash []byte = nil

	if didUserGivePassword {

		passHash, err = bcrypt.GenerateFromPassword([]byte(upload.Password), f.bcryptCost)

		if err != nil {
			return nil, err
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
			ActiveDownloads:  0,
			AllowedDownloads: upload.AllowedDownloads,
			UserSetPassword:  didUserGivePassword,
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
