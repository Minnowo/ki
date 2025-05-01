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

type FileMap struct {
	TempDir    string
	bcryptCost int
	keySize    crypto.AESKeySize
	files      map[FileID]*SafeFileEx
	l          sync.RWMutex
}

func NewFileMap(keySize crypto.AESKeySize, bcryptCost int) FileMap {
	return FileMap{
		files:      make(map[FileID]*SafeFileEx),
		keySize:    keySize,
		bcryptCost: bcryptCost,
	}
}

func (f *FileMap) GetFile(key FileID) *SafeFile {

	v, ok := f.get(key)

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

func (f *FileMap) RemoveExpired() {

	f.l.RLock()

	expired := make([]FileID, 0, len(f.files))

	for key, value := range f.files {
		if value.CleanIfExpired() && value.Clean() {
			expired = append(expired, key)
		}
	}

	f.l.RUnlock()

	if len(expired) <= 0 {
		return
	}

	log.Info().Int("count", len(expired)).Msg("removing expired files")

	f.l.Lock()
	defer f.l.Unlock()

	for _, key := range expired {

		_, ok := f.files[key]

		if ok {
			delete(f.files, key)
		}
	}
}

func (f *FileMap) ReadFile(w io.Writer, key FileID, password string) error {

	file, ok := f.get(key)

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

	download, err := file.StartDownload(password)

	if err == nil {
		defer file.StopDownload()
		defer download.File.Close()
	} else {
		return err
	}

	if r, ok := w.(http.ResponseWriter); ok {

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

func (f *FileMap) SaveFile(upload SafeFileUpload, r io.Reader) (*FileID, error) {
	return f.SaveFileWithProgress(upload, r, nil)
}

func (f *FileMap) SaveFileWithProgress(upload SafeFileUpload, r io.Reader, update func(int)) (*FileID, error) {

	if err := upload.Valid(); err != nil {
		return nil, err
	}

	file, err := os.CreateTemp(f.TempDir, config.FILENAME_PREFIX+"*")

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
		aesw, err = crypto.GetStreamEncryptionWriterEx(f.keySize, key, []byte(upload.Password), file)
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

		n, err := r.Read(buffer)

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

	var fileId FileID
	for {
		rand.Read(fileId[:])

		if _, ok := f.files[fileId]; !ok {
			break
		}
	}

	f.files[fileId] = &SafeFileEx{
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
	}

	log.Info().
		Str("name", file.Name()).
		Str("expires", upload.ExpiresIn.String()).
		Int("allowedDownloads", upload.AllowedDownloads).
		Int64("size", fileSize).
		Msg("saved new file")

	return &fileId, nil
}

func (f *FileMap) get(key FileID) (*SafeFileEx, bool) {
	f.l.RLock()
	defer f.l.RUnlock()
	v, ok := f.files[key]
	return v, ok
}
