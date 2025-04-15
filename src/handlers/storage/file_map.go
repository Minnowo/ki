package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"ki/src/config"
	"os"
	"sync"
	"time"

	"ki/src/handlers/crypto"

	"github.com/rs/zerolog/log"
)

type SafeFile struct {
	Hash             []byte
	Size             int64
	Expires          time.Time
	AllowedDownloads int
	Downloads        int
	Name             string
}

func (f *SafeFile) IsExpired() bool {
	return time.Now().After(f.Expires) || f.Downloads >= f.AllowedDownloads
}

func (f *SafeFile) Copy() *SafeFile {
	var sf SafeFile
	sf.Hash = make([]byte, len(f.Hash))
	copy(sf.Hash, f.Hash)
	sf.Size = f.Size
	sf.Expires = f.Expires
	sf.AllowedDownloads = f.AllowedDownloads
	sf.Downloads = f.Downloads
	sf.Name = f.Name
	return &sf
}

type SafeFileEx struct {
	SafeFile
	sync.RWMutex
	key        []byte
	file       *os.File
	wasCleaned bool
}

func (f *SafeFileEx) CleanIfExpired() bool {

	f.RLock()
	didExpire := f.SafeFile.IsExpired()
	wasCleaned := f.wasCleaned
	f.RUnlock()

	if !didExpire {
		return false
	}

	if wasCleaned {
		return true
	}

	f.Lock()

	if !f.wasCleaned {

		f.wasCleaned = true
		f.Expires = time.Unix(0, 0)

		if f.key != nil {
			rand.Read(f.key)
		}

		if f.file != nil {
			f.file.Close()
			os.Remove(f.file.Name())
			f.file = nil
		}
	}

	f.Unlock()

	return true
}

type FileMap struct {
	files map[FileID]*SafeFileEx
	sync.RWMutex
}

func NewFileMap() FileMap {
	return FileMap{
		files: make(map[FileID]*SafeFileEx),
	}
}

func (f *FileMap) GetFile(key FileID) *SafeFile {

	v, ok := f.get(key)

	if !ok {
		return nil
	}

	v.RLock()
	defer v.RUnlock()

	if v.IsExpired() {
		log.Info().Str("name", v.Name).Msg("file has expired")
		f.remove(key)
		return nil
	}

	return v.SafeFile.Copy()
}

func (f *FileMap) RemoveExpired() {

	f.RLock()

	expired := make([]FileID, len(f.files))

	for key, value := range f.files {
		if value.CleanIfExpired() {
			expired = append(expired, key)
		}
	}

	f.RUnlock()

	if len(expired) <= 0 {
		return
	}

	f.Lock()
	defer f.Unlock()

	for _, key := range expired {

		_, ok := f.files[key]

		if ok {
			delete(f.files, key)
		}
	}
}

func (f *FileMap) ReadFile(w io.Writer, key FileID) error {

	file, ok := f.get(key)

	if !ok {
		return fmt.Errorf("not found")
	}

	if file.CleanIfExpired() {
		f.remove(key)
		return fmt.Errorf("expired")
	}

	file.Lock()
	defer file.Unlock()

	if file.IsExpired() {
		return fmt.Errorf("expired")
	}

	if _, err := file.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("file error")
	}

	aesr, err := crypto.GetStreamDecryptionWriter(file.key, file.file)

	if err != nil {
		return err
	}

	file.Downloads += 1

	n, err := io.Copy(w, aesr)

	log.Info().Int64("n", n).Msg("wrote bytes to client")

	if err != nil {
		log.Warn().Err(err).Msg("error while sending someone a file")
		return err
	}

	return nil
}

func (f *FileMap) SaveFile(r io.Reader, expirey time.Time, allowedDownloads int, ogName string) (*FileID, error) {

	key := make([]byte, config.AES_KEY_SIZE)
	rand.Read(key)

	file, err := os.CreateTemp("", config.FILENAME_PREFIX+"*")

	if err != nil {
		return nil, err
	}

	aesw, err := crypto.GetStreamEncryptionWriter(key, file)

	if err != nil {
		return nil, err
	}

	hash := sha256.New()

	fileSize := int64(0)
	buffer := make([]byte, 1024*1024)
	for {

		n, err := r.Read(buffer)

		if n > 0 {
			fileSize += int64(n)
			aesw.Write(buffer[0:n])
			hash.Write(buffer[0:n])
		} else if n == 0 {
			break
		}

		if err != nil {

			if err == io.EOF {
				break
			}
			return nil, err
		}
	}

	var sha [sha256.Size]byte
	copy(sha[:], hash.Sum(nil))

	f.Lock()
	defer f.Unlock()

	var fileId FileID
	for {
		rand.Read(fileId[:])

		if _, ok := f.files[fileId]; !ok {
			break
		}
	}

	f.files[fileId] = &SafeFileEx{
		key:  key,
		file: file,
		SafeFile: SafeFile{
			Hash:             sha[:],
			Expires:          expirey,
			Size:             fileSize,
			Name:             ogName,
			Downloads:        0,
			AllowedDownloads: allowedDownloads,
		},
	}

	log.Info().
		Str("name", file.Name()).
		Str("expires", expirey.String()).
		Int("allowedDownloads", allowedDownloads).
		Int64("size", fileSize).
		Msg("saved new file")

	return &fileId, nil
}

func (f *FileMap) remove(key FileID) {
	f.Lock()
	defer f.Unlock()
	delete(f.files, key)
}

func (f *FileMap) get(key FileID) (*SafeFileEx, bool) {
	f.RLock()
	defer f.RUnlock()
	v, ok := f.files[key]
	return v, ok
}
