package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha256"
	"io"
)

type AESKeySize int

const (
	AES256 AESKeySize = 32
	AES192 AESKeySize = 24
	AES128 AESKeySize = 16
)

func (k AESKeySize) Assert() {
	switch k {
	case AES256:
		return
	case AES192:
		return
	case AES128:
		return
	default:
		panic("unreachable")
	}
}

const PBKDF2_ROUNDS int = 4096

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

	AESKeySize(len(key)).Assert()

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	var iv [aes.BlockSize]byte
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

	AESKeySize(len(key)).Assert()

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	var iv [aes.BlockSize]byte
	stream := cipher.NewCTR(block, iv[:])
	reader := &cipher.StreamReader{S: stream, R: r}

	return reader, nil
}
