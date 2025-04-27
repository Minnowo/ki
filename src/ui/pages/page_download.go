package pages

import (
	"ki/src/handlers/storage"
)

type PageDownloadFileView struct {
	BaseView
	File   *storage.SafeFile
	FileID storage.FileID
}
