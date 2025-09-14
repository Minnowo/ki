package storage

import (
	"crypto/rand"
	"io"
	"ki/src/handlers/crypto"
	"ki/src/pkg/bytes"
	"os"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// KiMetadata stores the basic information about a file in the store.
type KiMetadata struct {
	Sha512Hash       []byte    `cbor:"sha512"`
	Sha256Hash       []byte    `cbor:"sha256"`
	Sha1Hash         []byte    `cbor:"sha1"`
	Md5Hash          []byte    `cbor:"md5"`
	Size             int64     `cbor:"size"`
	Expires          time.Time `cbor:"expires"`
	AllowedDownloads int       `cbor:"dlallow"`
	Downloads        int       `cbor:"dl"`
	ActiveDownloads  int       `cbor:"dlactive"`
	Name             string    `cbor:"name"`
	UserSetPassword  bool      `cbor:"haspassword"`
	MemoryOnly       bool      `cbor:"memoryonly"`
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
		MemoryOnly:       f.MemoryOnly,
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

type KiFile struct {
	KiMetadata `cbor:"metadata"`

	// The path to the encrypted file on disk.
	FilePath string `cbor:"path"`

	// The key used for encrypting/decrypting the file.
	Key []byte `cbor:"key"`

	// If the user set a password, this is a bcrypt hash of the password.
	// The user's password will then be combined with the key to create the real key.
	UserPasswordHash []byte `cbor:"password"`

	// Set true if the file was deleted from disk.
	WasCleaned bool `cbor:"cleaned"`

	KeySize crypto.AESKeySize `cbor:"keysize"`
}

// Clone returns deep copy this file. This does not copy the file on disk.
func (f *KiFile) Clone() *KiFile {
	return &KiFile{
		KiMetadata:       *f.KiMetadata.Clone(),
		FilePath:         f.FilePath,
		Key:              bytes.CopyBytes(f.Key),
		UserPasswordHash: bytes.CopyBytes(f.UserPasswordHash),
		WasCleaned:       f.WasCleaned,
		KeySize:          f.KeySize,
	}
}

// Returns the metadata for this file.
func (f *KiFile) Metadata() *KiMetadata { return &f.KiMetadata }

// IsExpired returns if this file is expired, and should be deleted.
func (f *KiFile) IsExpired() bool {
	return f.KiMetadata.IsExpired()
}

// PasswordIsValid checks if this file's password (if any), matches the given password.
func (f *KiFile) PasswordIsValid(password string) bool {

	if err := bcrypt.CompareHashAndPassword(f.UserPasswordHash, []byte(password)); err != nil {

		if err != bcrypt.ErrMismatchedHashAndPassword {
			log.Warn().Err(err).Msg("error while comparing user password")
		}

		return false
	}

	return true
}

// NewReader gets a new reader for this file's data.
// Reader should be valid even if the file is expired.
// The caller is in charge of making sure this stream is not used on expired files.
func (f *KiFile) NewReader(password string) (io.ReadCloser, error) {

	fileHandle, err := os.Open(f.FilePath)

	if err != nil {
		return nil, err
	}

	var aesr io.Reader

	if f.UserSetPassword {
		aesr, err = crypto.GetStreamDecryptionReaderEx(f.KeySize, f.Key, password, fileHandle)
	} else {
		aesr, err = crypto.GetStreamDecryptionReader(f.Key, fileHandle)
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

// Clean expires this file and deletes all information about it.
// Clean will never delete the file as long as ActiveDownloads > 0.
// Returns true if the file was deleted from disk, otherwise false.
func (f *KiFile) Clean() bool {

	if f.WasCleaned {
		return true
	}

	log.Debug().Str("name", f.Name).Int("activeDownloads", f.ActiveDownloads).Msg("expiring file")

	rand.Read(f.Key)
	rand.Read(f.Sha1Hash)
	rand.Read(f.Sha256Hash)
	rand.Read(f.Sha512Hash)
	f.Expires = time.Unix(0, 0)
	f.Name = ""
	f.Size = 0

	if f.UserPasswordHash != nil {
		rand.Read(f.UserPasswordHash)
	}

	if f.ActiveDownloads > 0 {
		return false
	}

	// we want to keep trying to clean until the file is removed
	if err := os.Remove(f.FilePath); err != nil {

		if os.IsNotExist(err) {
			f.WasCleaned = true
		} else {
			log.Warn().Err(err).Msg("failed to remove file")
		}

	} else {
		f.WasCleaned = true
	}

	if f.WasCleaned {
		log.Info().Str("path", f.FilePath).Msg("file was cleaned")
		return true
	}
	return false
}

func (f *KiFile) ToBinary() (data []byte, err error) {
	return cbor.Marshal(f)

}

func (f *KiFile) FromBinary(data []byte) error {
	return cbor.Unmarshal(data, f)
}
