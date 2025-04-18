package config

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

const MAX_UPLOAD_SIZE int64 = 8000 * MB

type AESKeySize int

const (
	AES256 AESKeySize = 32
	AES192 AESKeySize = 24
	AES128 AESKeySize = 16
)
const AES_KEY_SIZE AESKeySize = AES256

const FILE_ID_SIZE int = 16

const FILENAME_PREFIX string = ""

const SHOW_DOWNLOAD_EXPIRE_TIME bool = true
const SHOW_DOWNLOAD_EXPIRE_TIME_REMAINING bool = true
const SHOW_DOWNLOADS_REMAINING bool = true

const EXPIREY_TIME_FORMAT string = "2006-01-02 15:04:05 MST"

const HOST string = "localhost"
