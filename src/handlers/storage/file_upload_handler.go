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
	FileDir              string
	bcryptCost           int
	keySize              crypto.AESKeySize
	metadataStore        FileStore
	daemonCloseChan      chan bool
	uploadSessionStore   *UploadSessionStore
	downloadSessionStore *DownloadSessionStore
}

func NewFileStore(dir string, store FileStore, bcryptCost int) FileUploadHandler {
	return FileUploadHandler{
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
				f.uploadSessionStore.ClearExpired()
				f.clearExpiredDownloadSessions()

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

		if file.UserSetPassword && !file.PasswordIsValid(password) {
			return ErrNeedsAuth
		}

		file.AddDownloader()

		return nil
	})

	// file expired, or the user password was wrong
	if err != nil {
		return err
	}

	// We need to clear the ActiveDownload we put from the above call
	defer f.metadataStore.WithFile(key, func(file *KiFile) error {

		file.SubDownloader()

		if file.ActiveDownloads < 0 {
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

		r.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))

		if file.Metadata().Name == "" {
			r.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", key.Hex()))
		} else {
			r.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", file.Name))
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

// BeginChunkedDownload validates the file password, opens a new DownloadSession.
// The returned FileID can be used to call ReadNextChunk to download the file.
// If the returned error is nil, then the FileID and *KiMetadata should not be used.
func (f *FileUploadHandler) BeginChunkedDownload(fileID FileID, password string) (FileID, *KiMetadata, error) {

	file, err := f.metadataStore.WithFile(fileID, func(file *KiFile) error {

		if file.IsExpired() {
			return ErrFileExpired
		}

		if file.UserSetPassword && !file.PasswordIsValid(password) {
			return ErrNeedsAuth
		}

		file.AddDownloader()

		return nil
	})

	if err != nil {
		return FileID{}, nil, err
	}

	stream, err := file.NewReader(password)

	if err != nil {
		// Undo the AddDownloader we just committed.
		f.metadataStore.WithFile(fileID, func(file *KiFile) error {
			file.SubDownloader()
			return nil
		})
		return FileID{}, nil, err
	}

	var sessionID FileID
	rand.Read(sessionID[:])

	f.downloadSessionStore.add(sessionID, &DownloadSession{
		Stream:       stream,
		TotalBytes:   file.Size,
		FileID:       fileID,
		LastActivity: time.Now(),
		buf:          make([]byte, config.DOWNLOAD_BUFFER_SIZE),
	})

	log.Info().Hex("id", sessionID[:]).Hex("file", fileID[:]).Msg("began chunked download session")

	return sessionID, file.Metadata(), nil
}

// ReadNextChunk delivers the next buffer-sized slice of the file to w.
// If the write to w fails, or the end of chunk is reached, the next call will resume writing from the last written byte.
// When the stream is exhausted, the session is automatically closed.
// Returns the number of bytes written, which should be used before an error.
func (f *FileUploadHandler) ReadNextChunk(w io.Writer, sessionID FileID) (int64, error) {

	session, ok := f.downloadSessionStore.get(sessionID)

	if !ok {
		return 0, ErrSessionNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	bytesWritten := int64(0)

	err := func() error {
		for {
			n, err := io.CopyN(w, session, config.MaxChunkSize())

			bytesWritten += n

			if n == 0 {
				return err
			}

			if bytesWritten >= config.MaxChunkSize() {
				return nil
			}

			if err != nil {
				return err
			}
		}
	}()

	session.BytesRead += bytesWritten
	session.LastActivity = time.Now()

	if session.BytesRead >= session.TotalBytes {
		f.closeDownloadSession(sessionID, session)
	}

	return bytesWritten, err
}

// AbortChunkedDownload cancels an in-progress download session, closing the stream.
func (f *FileUploadHandler) AbortChunkedDownload(sessionID FileID) error {

	session, ok := f.downloadSessionStore.get(sessionID)

	if !ok {
		return ErrSessionNotFound
	}

	f.closeDownloadSession(sessionID, session)

	log.Info().Hex("id", sessionID[:]).Msg("aborted chunked download session")

	return nil
}

func (f *FileUploadHandler) closeDownloadSession(sessionID FileID, session *DownloadSession) {

	log.Info().Hex("id", sessionID[:]).Msg("download stream closed")

	f.downloadSessionStore.remove(sessionID)

	session.Stream.Close()

	f.metadataStore.WithFile(session.FileID, func(file *KiFile) error {
		file.SubDownloader()
		return nil
	})
}

func (f *FileUploadHandler) clearExpiredDownloadSessions() {

	for _, session := range f.downloadSessionStore.removeExpired() {

		session.Stream.Close()

		f.metadataStore.WithFile(session.FileID, func(file *KiFile) error {
			file.SubDownloader()
			return nil
		})
	}
}

// SaveFile uploads the given file into the file store.
func (f *FileUploadHandler) SaveFile(upload FileUpload) (*FileID, error) {
	return f.SaveFileWithProgress(upload, nil)
}

// SaveFileWithProgress uploads the given file into the file store calling the update method
// with the number of bytes read as the file is processed.
func (f *FileUploadHandler) SaveFileWithProgress(upload FileUpload, update func(int)) (*FileID, error) {

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

func (f *FileUploadHandler) saveFileWithProgress(file *os.File, upload FileUpload, update func(int)) (*FileID, error) {

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
			ActiveDownloads:  0,
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
