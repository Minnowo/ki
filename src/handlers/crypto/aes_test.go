package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testReadCloser struct {
	io.Reader
	Closed bool
}

func (trc *testReadCloser) Close() error {
	trc.Closed = true
	return nil
}

func TestGetStreamEncryptionWriterEx(t *testing.T) {

	serverKey := make([]byte, 32)
	rand.Read(serverKey)

	userKey := "testpassword"
	plainText := []byte("This is a test message")

	tests := []struct {
		name    string
		keySize AESKeySize
	}{
		{"AES128", AES128},
		{"AES192", AES192},
		{"AES256", AES256},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			var buf bytes.Buffer

			// writer tests
			writer, err := GetStreamEncryptionWriterEx(tt.keySize, serverKey, userKey, &buf)
			assert.Nil(t, err, "failed to get writer")

			_, err = writer.Write(plainText)
			assert.Nil(t, err, "error writing plaintext")
			assert.LessOrEqual(t, aes.BlockSize, buf.Len(), "buffer should at least contain the IV")

			// reader tests
			reader, err := GetStreamDecryptionReaderEx(tt.keySize, serverKey, userKey, &buf)
			assert.Nil(t, err, "failed to get reader")

			decrypted := make([]byte, len(plainText))
			n, err := reader.Read(decrypted)
			assert.Nil(t, err, "error reading plaintext")
			assert.Equal(t, len(plainText), n, "read wrong number of bytes")
			assert.Equal(t, plainText, decrypted, "plainText and decrypted should match")
		})
	}
}

func TestGetStreamEncryptionWriter(t *testing.T) {

	key := make([]byte, 32)
	rand.Read(key)

	var buf bytes.Buffer
	writer, err := GetStreamEncryptionWriter(key, &buf)
	assert.Nil(t, err, "should be able to GetStreamEncryptionWriter")

	plainText := []byte("This is a test message")
	_, err = writer.Write(plainText)
	assert.Nil(t, err, "failed to write plainText to cipher")
	assert.LessOrEqual(t, aes.BlockSize, buf.Len(), "buffer should at least contain the IV")
}

func TestGetStreamDecryptionReader(t *testing.T) {

	key := make([]byte, 32)
	rand.Read(key)

	plainText := []byte("This is a test message")

	var buf bytes.Buffer
	writer, err := GetStreamEncryptionWriter(key, &buf)
	assert.Nil(t, err, "failed to GetStreamEncryptionWriter")

	_, err = writer.Write(plainText)
	assert.Nil(t, err, "failed to write ")

	reader, err := GetStreamDecryptionReader(key, &buf)
	assert.Nil(t, err, "failed to GetStreamDecryptionReader")

	decrypted := make([]byte, len(plainText))
	n, err := reader.Read(decrypted)
	assert.Nil(t, err, "failed to read")

	assert.Equal(t, len(plainText), n, "wrong size")
	assert.Equal(t, plainText, decrypted, "wrong data")
}

func TestCipherCloseReader(t *testing.T) {

	key := make([]byte, 32)
	rand.Read(key)

	plainText := []byte("This is a test message")

	var buf bytes.Buffer
	writer, err := GetStreamEncryptionWriter(key, &buf)
	assert.Nil(t, err, "failed to GetStreamEncryptionWriter")

	_, err = writer.Write(plainText)
	assert.Nil(t, err, "failed to write plainText")

	reader, err := GetStreamDecryptionReader(key, &buf)
	assert.Nil(t, err, "failed to GetStreamDecryptionReader")

	closerTest := &testReadCloser{Reader: &buf, Closed: false}
	// Wrap reader in CipherCloseReader
	closer := &CipherCloseReader{
		UnderStream:  closerTest,
		CipherStream: reader,
	}

	decrypted := make([]byte, len(plainText))
	n, err := closer.Read(decrypted)
	assert.Nil(t, err, "failed to read")

	assert.Equal(t, len(plainText), n, "didn't read the correct number of bytes")
	assert.Equal(t, plainText, decrypted, "decrypted did not match plainText")

	err = closer.Close()
	assert.Nil(t, err, "expected no error closing stream")
	assert.True(t, closerTest.Closed, "expected under stream to be closed")
}

func TestInvalidKeySize(t *testing.T) {

	var buf bytes.Buffer
	invalidKey := make([]byte, 31) // Invalid key size

	_, err := GetStreamEncryptionWriter(invalidKey, &buf)
	assert.NotNil(t, err, "should get error for invalid key size")

	_, err = GetStreamDecryptionReader(invalidKey, &buf)
	assert.NotNil(t, err, "should get error for invalid key size")
}

func TestInvalidIV(t *testing.T) {

	key := make([]byte, 32)
	rand.Read(key)

	var buf bytes.Buffer
	// Write incomplete IV
	_, err := buf.Write(make([]byte, aes.BlockSize-1))
	assert.Nil(t, err, "failed to write iv")

	_, err = GetStreamDecryptionReader(key, &buf)
	assert.NotNil(t, err, "expepcted error for invalid IV")
}
