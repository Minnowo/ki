package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
)

func GetStreamEncryptionWriter(key []byte, w io.Writer) (io.Writer, error) {

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	var iv [aes.BlockSize]byte
	stream := cipher.NewCTR(block, iv[:])
	writer := &cipher.StreamWriter{S: stream, W: w}

	return writer, nil
}

func GetStreamDecryptionWriter(key []byte, r io.Reader) (io.Reader, error) {

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	var iv [aes.BlockSize]byte
	stream := cipher.NewCTR(block, iv[:])
	reader := &cipher.StreamReader{S: stream, R: r}

	return reader, nil
}
