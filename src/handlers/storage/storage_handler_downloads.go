package storage

import (
	"fmt"
	"io"
	"ki/src/config"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

func (f *StorageHandler) ReadFile(w io.Writer, key FileID, password string) error {

	var fstream io.ReadCloser

	defer func() {
		if fstream != nil {
			fstream.Close()
		}
	}()

	file, err := f.metadataStore.WithFile(key, func(file *KiFile) error {

		if file.IsExpired() {
			return ErrFileExpired
		}

		if file.UserSetPassword && !file.PasswordIsValid(password) {
			return ErrNeedsAuth
		}

		// don't even need to track active downloads here.
		// having the file handle is enough to prevent deletion of the file.
		file.CountDownload()

		if handle, err := file.NewReader(password); err != nil {
			return err
		} else {
			fstream = handle
		}

		return nil
	})

	// file expired, or the user password was wrong
	if err != nil {
		return err
	}

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

	var fstream io.ReadCloser

	file, err := f.metadataStore.WithFile(fileID, func(file *KiFile) error {

		if file.IsExpired() {
			return ErrFileExpired
		}

		if file.UserSetPassword && !file.PasswordIsValid(password) {
			return ErrNeedsAuth
		}

		file.CountDownload()

		if handle, err := file.NewReader(password); err != nil {
			return err
		} else {
			fstream = handle
		}

		return nil
	})

	if err != nil {

		// worth checking this here in case the underlying store implementation returns an error that it shouldn't.
		if fstream != nil {
			fstream.Close()
		}

		return SessionToken{}, nil, err
	}

	var sessionID SessionToken
	sessionID.New()

	f.downloadSessionStore.add(sessionID, &DownloadSession{
		Stream:       fstream,
		TotalBytes:   file.Size,
		FileID:       fileID,
		File:         *file.Metadata(),
		LastActivity: time.Now(),
		buf:          make([]byte, config.DOWNLOAD_BUFFER_SIZE),
	})

	log.Info().Hex("id", sessionID[:]).Hex("file", fileID[:]).Msg("began chunked download session")

	return sessionID, file.Metadata(), nil
}

func (f *StorageHandler) WithDownloadSession(sessionID SessionToken, fileID FileID, callback func(session *DownloadSession) error) error {

	session, ok := f.downloadSessionStore.get(sessionID)

	if !ok {
		return ErrSessionNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.FileID != fileID {
		return ErrSessionNotFound
	}

	err := callback(session)

	if session.BytesWritten >= session.TotalBytes {
		f.closeDownloadSession(sessionID, session)
	}

	return err
}

// AbortDownloadSession cancels an in-progress download session, closing the stream.
func (f *StorageHandler) AbortDownloadSession(sessionID SessionToken, fileID FileID) error {

	session, ok := f.downloadSessionStore.get(sessionID)

	if !ok {
		return ErrSessionNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.FileID != fileID {
		return ErrSessionNotFound
	}

	f.closeDownloadSession(sessionID, session)

	log.Info().Hex("id", sessionID[:]).Msg("aborted chunked download session")

	return nil
}

func (f *StorageHandler) closeDownloadSession(sessionID SessionToken, session *DownloadSession) {

	log.Info().Hex("id", sessionID[:]).Msg("download stream closed")

	session.Stream.Close()

	f.downloadSessionStore.remove(sessionID)
}
