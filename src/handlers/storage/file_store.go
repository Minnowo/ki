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
	"sync"
	"time"

	"ki/src/handlers/crypto"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNeedsAuth   error = fmt.Errorf("needs password to download")
	ErrFileExpired error = fmt.Errorf("file has expired")
)

type FileStore struct {
	FileDir         string
	bcryptCost      int
	keySize         crypto.AESKeySize
	metadataStore   FileMetadataStore
	daemonCloseChan chan bool
	l               sync.RWMutex
}

func NewFileStore(bcryptCost int) FileStore {
	return FileStore{
		metadataStore:   NewMemoryFileMetadataStore(),
		keySize:         config.AES_KEY_SIZE,
		bcryptCost:      bcryptCost,
		daemonCloseChan: nil,
	}
}

func (f *FileStore) ShutdownExpireCheckLoop() {

	f.l.Lock()
	defer f.l.Unlock()

	if f.daemonCloseChan == nil {
		return
	}

	f.daemonCloseChan <- true
	close(f.daemonCloseChan)
	f.daemonCloseChan = nil
}

func (f *FileStore) RunExpireCheckLoop(interval time.Duration) {

	f.l.Lock()
	defer f.l.Unlock()

	if f.daemonCloseChan != nil {
		return
	}

	done := make(chan bool)

	f.daemonCloseChan = done

	log.Debug().Msg("file expire check daemon starting")

	go func() {

		ticker := time.NewTicker(interval)

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
}

func (f *FileStore) GetFile(key FileID) *SafeFile {

	v, ok := f.metadataStore.GetFileMetadata(key)

	if !ok {
		return nil
	}

	v.RLock()
	defer v.RUnlock()

	if v.IsExpired() {
		return nil
	}

	return v.SafeFile.Copy()
}

func (f *FileStore) ReadFile(w io.Writer, key FileID, password string) error {

	file, ok := f.metadataStore.GetFileMetadata(key)

	if !ok || file.CleanIfExpired() {
		return ErrFileExpired
	}

	if file.UserSetPassword {

		err := bcrypt.CompareHashAndPassword(file.userPasswordHash, []byte(password))

		if err != nil {

			if err != bcrypt.ErrMismatchedHashAndPassword {
				return err
			}

			return ErrNeedsAuth
		}
	}

	download, err := file.StartDownload(f.keySize, password)

	if err != nil {
		return err
	}

	defer file.StopDownload()
	defer download.File.Close()

	if r, ok := w.(http.ResponseWriter); ok {

		r.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))

		if download.Name == "" {
			r.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", key.Hex()))
		} else {
			r.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", download.Name))
		}
	}

	n, err := io.Copy(w, download.Reader)

	log.Debug().Int64("n", n).Msg("wrote bytes to client")

	if err != nil {
		log.Warn().Err(err).Msg("error while sending someone a file")
		return err
	}

	return nil
}

func (f *FileStore) SaveFile(upload FileUpload) (*FileID, error) {
	return f.SaveFileWithProgress(upload, nil)
}

func (f *FileStore) SaveFileWithProgress(upload FileUpload, update func(int)) (*FileID, error) {

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

	f.l.Lock()
	defer f.l.Unlock()

	fileId := f.metadataStore.StoreFileMetadata(&SafeFileEx{
		userPasswordHash: passHash,
		key:              key,
		filePath:         file.Name(),
		SafeFile: SafeFile{
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
		},
	})

	log.Info().
		Str("name", file.Name()).
		Str("expires", upload.ExpiresIn.String()).
		Int("allowedDownloads", upload.AllowedDownloads).
		Int64("size", fileSize).
		Msg("saved new file")

	return &fileId, nil
}
