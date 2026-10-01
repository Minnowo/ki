package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha256"
	"errors"
	"io"
)

var ErrInvalidOffset = errors.New("invalid offset")

// SeekableDecryptionReader decrypts a stream written by GetStreamEncryptionWriter,
// and can move to any position in the decrypted data.
//
// In CTR mode, the keystream for block n is the encryption of IV + n, so moving to a position
// only needs the file to be seeked and the counter to be recomputed.
type SeekableDecryptionReader struct {
	file   io.ReadSeekCloser
	block  cipher.Block
	iv     [aes.BlockSize]byte
	stream cipher.Stream
}

func GetSeekableDecryptionReaderEx(kSize AESKeySize, serverKey []byte, userKey string, file io.ReadSeekCloser) (*SeekableDecryptionReader, error) {

	key, err := pbkdf2.Key(sha256.New, userKey, serverKey, PBKDF2_ROUNDS, int(kSize))

	if err != nil {
		return nil, err
	}

	return GetSeekableDecryptionReader(key, file)
}

func GetSeekableDecryptionReader(key []byte, file io.ReadSeekCloser) (*SeekableDecryptionReader, error) {

	if !AESKeySize(len(key)).Assert() {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	r := &SeekableDecryptionReader{
		file:  file,
		block: block,
	}

	// The IV is stored at the start of the cipher stream.
	if _, err := io.ReadFull(file, r.iv[:]); err != nil {
		return nil, err
	}

	r.stream = cipher.NewCTR(block, r.iv[:])

	return r, nil
}

func (r *SeekableDecryptionReader) Read(p []byte) (int, error) {

	n, err := r.file.Read(p)

	r.stream.XORKeyStream(p[:n], p[:n])

	return n, err
}

func (r *SeekableDecryptionReader) Close() error {
	return r.file.Close()
}

// SeekTo moves the reader so the next Read starts at the given position in the decrypted data.
func (r *SeekableDecryptionReader) SeekTo(offset int64) error {

	if offset < 0 {
		return ErrInvalidOffset
	}

	blockIndex := offset / aes.BlockSize

	// the IV takes up the first block of the file
	if _, err := r.file.Seek(aes.BlockSize+blockIndex*aes.BlockSize, io.SeekStart); err != nil {
		return err
	}

	counter := addToCounter(r.iv, uint64(blockIndex))

	r.stream = cipher.NewCTR(r.block, counter[:])

	// the offset may be part way into a block
	if skip := offset % aes.BlockSize; skip > 0 {
		if _, err := io.CopyN(io.Discard, r, skip); err != nil {
			return err
		}
	}

	return nil
}

// addToCounter adds n to the IV as a 128 bit big endian number, the same way cipher.NewCTR increments it.
func addToCounter(iv [aes.BlockSize]byte, n uint64) [aes.BlockSize]byte {

	carry := n

	for i := aes.BlockSize - 1; i >= 0 && carry > 0; i-- {
		sum := uint64(iv[i]) + carry&0xff
		iv[i] = byte(sum)
		carry = carry>>8 + sum>>8
	}

	return iv
}
