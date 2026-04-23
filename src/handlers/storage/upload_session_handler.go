package storage

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"io"
	"os"
	"strings"
	"time"

	"ki/src/config"
	"ki/src/handlers/crypto"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// BeginChunkedUpload creates an upload session, sets up encryption and hashing pipelines,
// and returns a session ID the client uses for subsequent chunk and complete requests.
func (f *FileUploadHandler) BeginChunkedUpload(upload FileUpload, username string) (FileID, error) {

	if err := upload.Valid(); err != nil {
		return FileID{}, err
	}

	var err error
	var passHash []byte

	if upload.Password != "" {

		passHash, err = bcrypt.GenerateFromPassword([]byte(upload.Password), f.bcryptCost)

		if err != nil {
			return FileID{}, err
		}
	}

	tmpFile, err := os.CreateTemp(f.FileDir, config.FILENAME_PREFIX+"*")

	if err != nil {
		return FileID{}, err
	}

	key := make([]byte, f.keySize)
	rand.Read(key)

	var cipherWriter io.Writer

	if upload.Password != "" {
		cipherWriter, err = crypto.GetStreamEncryptionWriterEx(f.keySize, key, upload.Password, tmpFile)
	} else {
		cipherWriter, err = crypto.GetStreamEncryptionWriter(key, tmpFile)
	}

	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return FileID{}, err
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
		Filename:         strings.TrimSpace(upload.Filename),
		ExpiresIn:        upload.ExpiresIn,
		AllowedDownloads: upload.AllowedDownloads,
		MemoryOnly:       upload.MemoryOnly,
		HasPassword:      upload.Password != "",
		PasswordHash:     passHash,
		CreatedAt:        now,
		LastActivity:     now,
		Username:         username,
	}

	var sessionID FileID
	rand.Read(sessionID[:])

	f.sessionStore.add(sessionID, session)

	log.Info().Hex("id", sessionID[:]).Str("username", username).Msg("began chunked upload session")

	return sessionID, nil
}

// AppendChunk reads up to limit bytes from r and appends them to the session's encrypted
// temp file while updating all running hash states. Chunks must be sent sequentially.
func (f *FileUploadHandler) AppendChunk(uploadId FileID, username string, r io.Reader, limit int64, update func(int)) (int64, error) {

	session, ok := f.sessionStore.get(uploadId)

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
					log.Info().Err(wErr).Msg("error while writing to the session")
					return wErr
				}

				if n != wN {
					return ErrChunkUploadTruncatedWrite
				}
			}

			if err != nil {

				if err == io.EOF {
					return nil
				}

				log.Info().Err(err).Msg("returning error from AppendChunk")

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

// CompleteChunkedUpload finalises the upload: closes the temp file, commits metadata to the
// file store, and returns the new FileID. The session is removed if the file is closed, regardless of if the metadata is put into the store. If adding the metadata into the filestore fails, the file is deleted.
func (f *FileUploadHandler) CompleteChunkedUpload(uploadId FileID, username string) (*FileID, error) {

	session, ok := f.sessionStore.get(uploadId)

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

	f.sessionStore.remove(uploadId)

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
			ActiveDownloads:  0,
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

// AbortChunkedUpload cancels an in-progress upload, deleting the temp file.
func (f *FileUploadHandler) AbortChunkedUpload(uploadId FileID, username string) error {

	session, ok := f.sessionStore.get(uploadId)

	if !ok {
		return ErrSessionNotFound
	}

	if session.Username != username {
		return ErrSessionNotFound
	}

	f.sessionStore.remove(uploadId)
	session.TempFile.Close()
	os.Remove(session.TempFile.Name())

	log.Info().Hex("id", uploadId[:]).Str("username", username).Msg("aborted chunked upload")

	return nil
}
