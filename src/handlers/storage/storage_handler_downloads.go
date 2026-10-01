package storage

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"ki/src/config"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

// openFile checks the password, opens a reader for the file, and then counts the download.
//
// The password check and opening the reader are slow (bcrypt and PBKDF2), so they are done on a copy of the file
// without holding the store's lock. A wrong password never takes the lock.
// This is safe because the password and key of a stored file never change. Only expiry can change in between,
// which is checked again when the download is counted.
func (f *StorageHandler) openFile(id FileID, password string) (*KiFile, FileStream, error) {

	file, ok := f.metadataStore.GetFile(id)

	if !ok {
		return nil, nil, ErrFileNotFound
	}

	if file.IsExpired() {
		return nil, nil, ErrFileExpired
	}

	if file.UserSetPassword && !file.PasswordIsValid(password) {
		return nil, nil, ErrNeedsAuth
	}

	// don't even need to track active downloads here.
	// having the file handle is enough to prevent deletion of the file.
	fstream, err := file.NewReader(password)

	if err != nil {

		// the file was cleaned after we got the copy
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, ErrFileExpired
		}

		return nil, nil, err
	}

	file, err = f.metadataStore.WithFile(id, func(file *KiFile) error {

		// another download may have used the last allowed download after we got the copy
		if file.IsExpired() {
			return ErrFileExpired
		}

		file.CountDownload()

		return nil
	})

	if err != nil {
		fstream.Close()
		return nil, nil, err
	}

	return file, fstream, nil
}

func (f *StorageHandler) ReadFile(w io.Writer, key FileID, password string) error {

	file, fstream, err := f.openFile(key, password)

	// file expired, or the user password was wrong
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
func (f *StorageHandler) BeginChunkedDownload(fileID FileID, password string) (SessionToken, *KiMetadata, error) {

	file, fstream, err := f.openFile(fileID, password)

	if err != nil {
		return SessionToken{}, nil, err
	}

	var sessionID SessionToken
	sessionID.New()

	session := &DownloadSession{
		Stream:     fstream,
		TotalBytes: file.Size,
		FileID:     fileID,
		File:       *file.Metadata(),
		buf:        make([]byte, config.DOWNLOAD_BUFFER_SIZE),
	}
	session.LastActivity.Touch()

	f.downloadSessionStore.add(sessionID, session)

	log.Info().Hex("id", sessionID[:]).Hex("file", fileID[:]).Msg("began chunked download session")

	return sessionID, file.Metadata(), nil
}

// WithDownloadSession calls callback with the session locked.
//
// The session stays open after the last byte is sent, so the final chunk can be retried.
// It is closed when the client calls AbortDownloadSession, or when it times out.
func (f *StorageHandler) WithDownloadSession(sessionID SessionToken, fileID FileID, callback func(session *DownloadSession) error) error {

	session, ok := f.downloadSessionStore.get(sessionID)

	if !ok {
		return ErrSessionNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// the session may have been closed while we were waiting for the lock
	if session.closed || session.FileID != fileID {
		return ErrSessionNotFound
	}

	return callback(session)
}

// AbortDownloadSession cancels an in-progress download session, closing the stream.
func (f *StorageHandler) AbortDownloadSession(sessionID SessionToken, fileID FileID) error {

	session, ok := f.downloadSessionStore.get(sessionID)

	if !ok {
		return ErrSessionNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// the session may have been closed while we were waiting for the lock
	if session.closed || session.FileID != fileID {
		return ErrSessionNotFound
	}

	f.closeDownloadSession(sessionID, session)

	log.Info().Hex("id", sessionID[:]).Msg("aborted chunked download session")

	return nil
}

func (f *StorageHandler) closeDownloadSession(sessionID SessionToken, session *DownloadSession) {

	log.Info().Hex("id", sessionID[:]).Msg("download stream closed")

	session.Close()

	f.downloadSessionStore.remove(sessionID)
}
