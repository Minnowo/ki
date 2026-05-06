package storage

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ki/src/config"
	"ki/src/handlers/crypto"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// CreateUploadSession creates an upload session, sets up encryption and hashing pipelines,
// and returns a session ID the client uses for subsequent chunk and complete requests.
func (f *StorageHandler) CreateUploadSession(fup FileUpload, username string) (SessionToken, error) {

	if err := fup.Valid(); err != nil {
		return SessionToken{}, err
	}

	var err error
	var passHash []byte

	if fup.Password != "" {

		passHash, err = bcrypt.GenerateFromPassword([]byte(fup.Password), f.bcryptCost)

		if err != nil {
			return SessionToken{}, err
		}
	}

	tmpFile, err := os.CreateTemp(f.FileDir, config.FILENAME_PREFIX+"*")

	if err != nil {
		return SessionToken{}, err
	}

	key := make([]byte, f.keySize)
	rand.Read(key)

	var cipherWriter io.Writer

	if fup.Password != "" {
		cipherWriter, err = crypto.GetStreamEncryptionWriterEx(f.keySize, key, fup.Password, tmpFile)
	} else {
		cipherWriter, err = crypto.GetStreamEncryptionWriter(key, tmpFile)
	}

	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return SessionToken{}, err
	}

	now := time.Now()
	session := &UploadSession{

		TempFile: tmpFile,

		CipherWriter: cipherWriter,
		SHA512H:      sha512.New(),
		SHA256H:      sha256.New(),
		SHA1H:        sha1.New(),
		MD5H:         md5.New(),

		AESKey:           key,
		Filename:         strings.TrimSpace(fup.Filename),
		ExpiresIn:        fup.ExpiresIn,
		AllowedDownloads: fup.AllowedDownloads,
		MemoryOnly:       fup.MemoryOnly,
		HasPassword:      fup.Password != "",
		PasswordHash:     passHash,
		CreatedAt:        now,
		LastActivity:     now,
		Username:         username,
	}

	var sessionID SessionToken
	sessionID.New()

	f.uploadSessionStore.add(sessionID, session)

	log.Info().Hex("id", sessionID[:]).Str("username", username).Msg("began chunked upload session")

	return sessionID, nil
}

// UploadSessionData reads up to limit bytes from r and appends them to the session's encrypted
// temp file while updating all running hash states. Chunks must be sent sequentially.
func (f *StorageHandler) UploadSessionData(sessionID SessionToken, username string, r io.Reader, limit int64, update func(int)) (int64, error) {

	session, ok := f.uploadSessionStore.get(sessionID)

	if !ok {
		return 0, ErrSessionNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Username != username {
		return 0, ErrSessionNotFound
	}

	bytesWritten := int64(0)

	err := (func() error {
		reader := io.LimitReader(r, limit)
		buffer := make([]byte, 4*config.KB)
		for {
			n, err := reader.Read(buffer)

			if n > 0 {

				wN, wErr := session.Write(buffer[0:n])

				bytesWritten += int64(wN)

				if update != nil {
					update(wN)
				}

				if wErr != nil {
					log.Error().Err(wErr).Msg("error writing data to disk")
					return wErr
				}

				if n != wN {
					return ErrTruncatedWrite
				}

				if session.TotalBytes+bytesWritten > config.MaxUploadSize() {

					log.Info().Hex("id", sessionID[:]).Str("username", username).Msg("max upload size exceeded")

					f.uploadSessionStore.remove(sessionID)
					session.Cleanup()

					return ErrMaxUploadSizeExceeded
				}
			}

			if err != nil {

				if err == io.EOF {
					return nil
				}

				if _, ok := err.(*http.MaxBytesError); ok {

					log.Info().Hex("id", sessionID[:]).Str("username", username).Msg("max upload size exceeded")

					f.uploadSessionStore.remove(sessionID)
					session.Cleanup()

					return ErrMaxUploadSizeExceeded
				}

				log.Debug().Err(err).Msg("returning error from AppendChunk")

				return err
			}

			if n == 0 {
				return nil
			}
		}
	})()

	session.TotalBytes += bytesWritten
	session.LastActivity = time.Now()

	return bytesWritten, err
}

// CompleteUploadSession finalises the upload: closes the temp file, commits metadata to the
// file store, and returns the new FileID. The session is removed if the file is closed, regardless of if the metadata is put into the store. If adding the metadata into the filestore fails, the file is deleted.
func (f *StorageHandler) CompleteUploadSession(uploadId SessionToken, username string) (*FileID, error) {

	session, ok := f.uploadSessionStore.get(uploadId)

	if !ok {
		return nil, ErrSessionNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Username != username {
		return nil, ErrSessionNotFound
	}

	if err := session.TempFile.Close(); err != nil {
		return nil, err
	}

	f.uploadSessionStore.remove(uploadId)

	fileId, err := f.metadataStore.StoreFile(&KiFile{
		UserPasswordHash: session.PasswordHash,
		Key:              session.AESKey,
		FilePath:         session.TempFile.Name(),
		KeySize:          f.keySize,
		KiMetadata: KiMetadata{
			Sha512Hash:       session.SHA512H.Sum(nil),
			Sha256Hash:       session.SHA256H.Sum(nil),
			Sha1Hash:         session.SHA1H.Sum(nil),
			Md5Hash:          session.MD5H.Sum(nil),
			Expires:          time.Now().Add(session.ExpiresIn),
			Size:             session.TotalBytes,
			Name:             session.Filename,
			Downloads:        0,
			AllowedDownloads: session.AllowedDownloads,
			UserSetPassword:  session.HasPassword,
			MemoryOnly:       session.MemoryOnly,
		},
	})

	if err != nil {
		os.Remove(session.TempFile.Name())
		return nil, err
	}

	log.Info().
		Hex("id", uploadId[:]).
		Str("username", username).
		Int64("size", session.TotalBytes).
		Msg("completed chunked upload")

	return &fileId, nil
}

// AbortUploadSession cancels an in-progress upload, deleting the temp file.
func (f *StorageHandler) AbortUploadSession(uploadId SessionToken, username string) error {

	session, ok := f.uploadSessionStore.get(uploadId)

	if !ok {
		return ErrSessionNotFound
	}

	if session.Username != username {
		return ErrSessionNotFound
	}

	f.uploadSessionStore.remove(uploadId)
	session.Cleanup()

	log.Info().Hex("id", uploadId[:]).Str("username", username).Msg("aborted chunked upload")

	return nil
}
