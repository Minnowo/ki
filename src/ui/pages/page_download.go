package pages

import (
	"ki/src/handlers/storage"
)

type PageDownloadView struct {
	File   *storage.SafeFile
	FileID storage.FileID
}
