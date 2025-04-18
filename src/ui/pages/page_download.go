package pages

import (
	"ki/src/handlers/storage"
)

type PageDownloadView struct {
	BaseView
	File   *storage.SafeFile
	FileID storage.FileID
}
