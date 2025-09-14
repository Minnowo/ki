package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

const PBKDF2_ROUNDS int = 4096
const PBKDF2_ROUNDS_FASTER int = 1024

var (
	ErrInvalidKeySize = fmt.Errorf("invalid AESKeySize")
)

type AESKeySize int

const (
	AES256 AESKeySize = 32
	AES192 AESKeySize = 24
	AES128 AESKeySize = 16
)

func (k AESKeySize) Assert() bool {
	switch k {
	case AES256:
		return true
	case AES192:
		return true
	case AES128:
		return true
	default:
		return false
	}
}

type CipherCloseReader struct {
	UnderStream  io.ReadCloser
	CipherStream io.Reader
}

func (c *CipherCloseReader) Read(p []byte) (int, error) {
	return c.CipherStream.Read(p)
}
func (c *CipherCloseReader) Close() error {
	return c.UnderStream.Close()
}

func GetStreamEncryptionWriterEx(kSize AESKeySize, serverKey []byte, userKey string, w io.Writer) (io.Writer, error) {

	key, err := pbkdf2.Key(sha256.New, userKey, serverKey, PBKDF2_ROUNDS, int(kSize))

	if err != nil {
		return nil, err
	}

	return GetStreamEncryptionWriter(key, w)
}

func GetStreamEncryptionWriter(key []byte, w io.Writer) (io.Writer, error) {

	if !AESKeySize(len(key)).Assert() {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	var iv [aes.BlockSize]byte

	if _, err := io.ReadFull(rand.Reader, iv[:]); err != nil {
		return nil, err
	}

	// The IV needs to be unique, but not secure.
	// We can store it at the start of the cipher stream.
	if _, err := w.Write(iv[:]); err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv[:])
	writer := &cipher.StreamWriter{S: stream, W: w}

	return writer, nil
}

func GetStreamDecryptionReaderEx(kSize AESKeySize, serverKey []byte, userKey string, r io.Reader) (io.Reader, error) {

	key, err := pbkdf2.Key(sha256.New, userKey, serverKey, PBKDF2_ROUNDS, int(kSize))

	if err != nil {
		return nil, err
	}

	return GetStreamDecryptionReader(key, r)
}

func GetStreamDecryptionReader(key []byte, r io.Reader) (io.Reader, error) {

	if !AESKeySize(len(key)).Assert() {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	var iv [aes.BlockSize]byte

	// We store the IV at the start of the cipher stream.
	// So we need to read it for decrpytion.
	if n, err := r.Read(iv[:]); err != nil || n != len(iv) {
		if err == nil {
			return nil, fmt.Errorf("could not read IV from stream")
		}
		return nil, err
	}

	// It's important to remember that ciphertexts must be authenticated
	// (i.e. by using crypto/hmac) as well as being encrypted in order to
	// be secure.
	//
	// We dot not do any authentication on the cipher stream.
	// The user uploading a file should check the hash before uploading, and compare after downloading.

	stream := cipher.NewCTR(block, iv[:])
	reader := &cipher.StreamReader{S: stream, R: r}

	return reader, nil
}

func DecryptBytes(kSize AESKeySize, key string, salt []byte, data []byte) ([]byte, error) {

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	subkey, err := pbkdf2.Key(sha256.New, key, salt, PBKDF2_ROUNDS_FASTER, int(kSize))

	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(subkey)

	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCMWithRandomNonce(block)

	if err != nil {
		return nil, err
	}

	return aesgcm.Open(nil, []byte{}, data, nil)
}

func EncryptBytes(kSize AESKeySize, key string, salt []byte, data []byte) ([]byte, error) {

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	subkey, err := pbkdf2.Key(sha256.New, key, salt, PBKDF2_ROUNDS_FASTER, int(kSize))

	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(subkey)

	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCMWithRandomNonce(block)

	if err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(nil, []byte{}, data, nil)

	return ciphertext, nil
}
