package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// encryptToFile writes plainText encrypted with GetStreamEncryptionWriter to a temp file, and opens it for reading.
func encryptToFile(t *testing.T, key []byte, plainText []byte) *os.File {
	t.Helper()

	path := filepath.Join(t.TempDir(), "encrypted")

	out, err := os.Create(path)
	assert.NoError(t, err)

	writer, err := GetStreamEncryptionWriter(key, out)
	assert.NoError(t, err)

	_, err = writer.Write(plainText)
	assert.NoError(t, err)
	assert.NoError(t, out.Close())

	in, err := os.Open(path)
	assert.NoError(t, err)
	t.Cleanup(func() { in.Close() })

	return in
}

func TestSeekableDecryptionReader(t *testing.T) {

	key := make([]byte, 32)
	rand.Read(key)

	plainText := make([]byte, 1000)
	rand.Read(plainText)

	t.Run("reads the whole stream", func(t *testing.T) {
		reader, err := GetSeekableDecryptionReader(key, encryptToFile(t, key, plainText))
		assert.NoError(t, err)

		decrypted, err := io.ReadAll(reader)
		assert.NoError(t, err)
		assert.Equal(t, plainText, decrypted)
	})

	t.Run("seeks to any position", func(t *testing.T) {
		reader, err := GetSeekableDecryptionReader(key, encryptToFile(t, key, plainText))
		assert.NoError(t, err)

		// block boundaries, part way into blocks, backwards and forwards, the start and the end
		for _, offset := range []int64{500, 0, 16, 17, 15, 999, 1000, 31, 32, 33, 250, 1} {
			assert.NoError(t, reader.SeekTo(offset))

			decrypted, err := io.ReadAll(reader)
			assert.NoError(t, err)
			assert.Equal(t, plainText[offset:], decrypted, "offset %d", offset)
		}
	})

	t.Run("seeks with a password derived key", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "encrypted")
		out, err := os.Create(path)
		assert.NoError(t, err)

		writer, err := GetStreamEncryptionWriterEx(AES256, key, "password", out)
		assert.NoError(t, err)
		_, err = writer.Write(plainText)
		assert.NoError(t, err)
		assert.NoError(t, out.Close())

		in, err := os.Open(path)
		assert.NoError(t, err)

		reader, err := GetSeekableDecryptionReaderEx(AES256, key, "password", in)
		assert.NoError(t, err)
		defer reader.Close()

		assert.NoError(t, reader.SeekTo(123))

		decrypted, err := io.ReadAll(reader)
		assert.NoError(t, err)
		assert.Equal(t, plainText[123:], decrypted)
	})

	t.Run("negative offset", func(t *testing.T) {
		reader, err := GetSeekableDecryptionReader(key, encryptToFile(t, key, plainText))
		assert.NoError(t, err)

		assert.ErrorIs(t, reader.SeekTo(-1), ErrInvalidOffset)
	})

	t.Run("missing IV", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "short")
		assert.NoError(t, os.WriteFile(path, make([]byte, aes.BlockSize-1), 0600))

		in, err := os.Open(path)
		assert.NoError(t, err)
		defer in.Close()

		_, err = GetSeekableDecryptionReader(key, in)
		assert.Error(t, err)
	})
}

// addToCounter must match how cipher.NewCTR increments the counter, including carrying between bytes.
func TestAddToCounter(t *testing.T) {

	key := make([]byte, 32)
	rand.Read(key)

	block, err := aes.NewCipher(key)
	assert.NoError(t, err)

	const blocks = 600

	ivs := map[string][aes.BlockSize]byte{
		"zero":             {},
		"carry one byte":   {15: 0xf0},
		"carry many bytes": {8: 0x01, 9: 0xff, 10: 0xff, 11: 0xff, 12: 0xff, 13: 0xff, 14: 0xff, 15: 0xff},
		"wrap around":      {0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe},
	}

	for name, iv := range ivs {
		t.Run(name, func(t *testing.T) {

			// the keystream cipher.NewCTR produces by stepping the counter itself
			keystream := make([]byte, blocks*aes.BlockSize)
			cipher.NewCTR(block, iv[:]).XORKeyStream(keystream, keystream)

			for n := range blocks {
				counter := addToCounter(iv, uint64(n))

				got := make([]byte, aes.BlockSize)
				cipher.NewCTR(block, counter[:]).XORKeyStream(got, got)

				want := keystream[n*aes.BlockSize : (n+1)*aes.BlockSize]

				if !bytes.Equal(want, got) {
					t.Fatalf("block %d: keystream does not match", n)
				}
			}
		})
	}
}
