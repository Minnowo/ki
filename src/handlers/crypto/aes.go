package crypto

import (
	"crypto/aes"
	"crypto/cipher"
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

func GetStreamEncryptionWriterEx(kSize AESKeySize, serverKey, userKey []byte, w io.Writer) (io.Writer, error) {

	key, err := OneTimePad(serverKey, userKey)

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

func GetStreamDecryptionReaderEx(serverKey, userKey []byte, r io.Reader) (io.Reader, error) {

	key, err := OneTimePad(serverKey, userKey)

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
