package storage

import (
	"crypto/rand"
	"encoding"
	"io"
	"ki/src/handlers/bytes"
	"ki/src/handlers/crypto"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type KiFile interface {

	// For storing this file inside a store.
	encoding.BinaryMarshaler

	// For reading this file from a store.
	encoding.BinaryUnmarshaler

	// Returns the metadata for this file.
	Metadata() *KiMetadata

	// Clone returns deep copy this file. This does not copy the file on disk.
	Clone() KiFile

	// IsExpired returns if this file is expired, and should be deleted.
	IsExpired() bool

	// Clean expires this file and deletes all information about it.
	// Clean will never delete the file as long as ActiveDownloads > 0.
	// Returns true if the file was deleted from disk, otherwise false.
	Clean() bool

	// PasswordIsValid checks if this file's password (if any), matches the given password.
	PasswordIsValid(password string) bool

	// NewReader gets a new reader for this file's data.
	// Reader should be valid even if the file is expired.
	// The caller is in charge of making sure this stream is not used on expired files.
	NewReader(password string) (io.ReadCloser, error)
}

// KiMetadata stores the basic information about a file in the store.
type KiMetadata struct {
	Sha512Hash       []byte
	Sha256Hash       []byte
	Sha1Hash         []byte
	Md5Hash          []byte
	Size             int64
	Expires          time.Time
	AllowedDownloads int
	Downloads        int
	ActiveDownloads  int
	Name             string
	UserSetPassword  bool
}

func (f *KiMetadata) Clone() *KiMetadata {
	return &KiMetadata{
		Sha512Hash:       bytes.CopyBytes(f.Sha512Hash),
		Sha256Hash:       bytes.CopyBytes(f.Sha256Hash),
		Sha1Hash:         bytes.CopyBytes(f.Sha1Hash),
		Md5Hash:          bytes.CopyBytes(f.Md5Hash),
		Size:             f.Size,
		Expires:          f.Expires,
		AllowedDownloads: f.AllowedDownloads,
		Downloads:        f.Downloads,
		ActiveDownloads:  f.ActiveDownloads,
		Name:             f.Name,
		UserSetPassword:  f.UserSetPassword,
	}
}

func (f *KiMetadata) IsExpired() bool {
	return time.Now().After(f.Expires) || f.Downloads >= f.AllowedDownloads
}

func (f *KiMetadata) AddDownloader() {
	f.ActiveDownloads++
	f.Downloads++
}
func (f *KiMetadata) SubDownloader() {
	f.ActiveDownloads--
}

type KiEncryptedFile struct {
	KiMetadata

	// The path to the encrypted file on disk.
	filePath string

	// The key used for encrypting/decrypting the file.
	key []byte

	// If the user set a password, this is a bcrypt hash of the password.
	// The user's password will then be combined with the key to create the real key.
	userPasswordHash []byte

	// Set true if the file was deleted from disk.
	wasCleaned bool

	keySize crypto.AESKeySize
}

func (f *KiEncryptedFile) Clone() KiFile {
	return &KiEncryptedFile{
		KiMetadata:       *f.KiMetadata.Clone(),
		filePath:         f.filePath,
		key:              bytes.CopyBytes(f.key),
		userPasswordHash: bytes.CopyBytes(f.userPasswordHash),
		wasCleaned:       f.wasCleaned,
		keySize:          f.keySize,
	}
}

func (f *KiEncryptedFile) Metadata() *KiMetadata { return &f.KiMetadata }

func (f *KiEncryptedFile) IsExpired() bool {
	return f.KiMetadata.IsExpired()
}

func (f *KiEncryptedFile) PasswordIsValid(password string) bool {

	if err := bcrypt.CompareHashAndPassword(f.userPasswordHash, []byte(password)); err != nil {

		if err != bcrypt.ErrMismatchedHashAndPassword {
			log.Warn().Err(err).Msg("error while comparing user password")
		}

		return false
	}

	return true
}

func (f *KiEncryptedFile) NewReader(password string) (io.ReadCloser, error) {

	fileHandle, err := os.Open(f.filePath)

	if err != nil {
		return nil, err
	}

	var aesr io.Reader

	if f.UserSetPassword {
		aesr, err = crypto.GetStreamDecryptionReaderEx(f.keySize, f.key, password, fileHandle)
	} else {
		aesr, err = crypto.GetStreamDecryptionReader(f.key, fileHandle)
	}

	if err != nil {
		fileHandle.Close()
		return nil, err
	}

	var fstream crypto.CipherCloseReader = crypto.CipherCloseReader{
		UnderStream:  fileHandle,
		CipherStream: aesr,
	}

	return &fstream, nil

}

func (f *KiEncryptedFile) Clean() bool {

	if f.wasCleaned {
		return true
	}

	log.Debug().Str("name", f.Name).Int("activeDownloads", f.ActiveDownloads).Msg("expiring file")

	rand.Read(f.key)
	rand.Read(f.Sha1Hash)
	rand.Read(f.Sha256Hash)
	rand.Read(f.Sha512Hash)
	f.Expires = time.Unix(0, 0)
	f.Name = ""
	f.Size = 0

	if f.userPasswordHash != nil {
		rand.Read(f.userPasswordHash)
	}

	if f.ActiveDownloads > 0 {
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

func (f *KiEncryptedFile) MarshalBinary() (data []byte, err error) {
	return data, err
}

func (f *KiEncryptedFile) UnmarshalBinary(data []byte) error {
	return nil
}
