package config

import (
	"ki/src/handlers/crypto"
	"time"
)

const (
	_  = iota //ignore first value by assigning to blank identifier
	KB = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
	ZB
	YB
)

const SESSION_MAX_AGE time.Duration = time.Hour * 24

const UPLOAD_TIMEOUT_PER_READ_SECONDS int64 = 60

const MAX_LOGIN_FORM_SIZE int64 = 2 * KB
const MAX_UPLOAD_SESSION_FORM_SIZE int64 = 4 * KB
const MAX_DOWNLOAD_FORM_SIZE int64 = 2 * KB

const MAX_USERNAME_LENGTH int = 10
const MAX_USER_PASSWORD_LENGTH int = 32

const AES_KEY_SIZE crypto.AESKeySize = crypto.AES256
const MAX_PASSWORD_LENGTH int = 72 // bcrypt max allowed

const FILENAME_PREFIX string = "ki_"

// MEMORY_FILE_DIR_NAME is the folder inside the file storage dir holding the files of memory only uploads.
// Their metadata is lost on restart, so the folder is cleared on startup.
const MEMORY_FILE_DIR_NAME string = "mem"

const SHOW_DOWNLOAD_EXPIRE_TIME bool = true
const SHOW_DOWNLOAD_EXPIRE_TIME_REMAINING bool = true
const SHOW_DOWNLOADS_REMAINING bool = true

const EXPIREY_TIME_FORMAT string = "2006-01-02 15:04:05 MST"

const HOST string = "localhost"

// DOWNLOAD_BUFFER_SIZE is the size of the per-session read buffer for chunked downloads.
const DOWNLOAD_BUFFER_SIZE int64 = 512 * KB

var (
	// Total upload for a single file.
	maxUploadSize int64 = 2 * GB

	// This is the max number of bytes per request.
	// If the file is larger than this, we will chunk the file.
	maxChunkSize   int64         = 50 * MB
	sessionTimeout time.Duration = 30 * time.Minute
)

func MaxUploadSize() int64 {
	return maxUploadSize
}

func SetMaxUploadSize(size int64) {
	maxUploadSize = size
}

func MaxChunkSize() int64 {
	return maxChunkSize
}

func SetMaxChunkSize(size int64) {
	if size > maxUploadSize {
		panic("cannot have the max chunk size larger than the max upload size")
	}
	maxChunkSize = size
}

func SessionTimeout() time.Duration {
	return sessionTimeout
}
