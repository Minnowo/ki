package v1

import (
	"bytes"
	"io"
	"ki/src/ui/formkeys"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {

	t.Run("working upload", func(t *testing.T) {
		assert := assert.New(t)

		// instantiate multipart request
		var buf bytes.Buffer
		multipartWriter := multipart.NewWriter(&buf)

		var part io.Writer
		var err error
		assert.Nil(multipartWriter.WriteField(formkeys.UPLOAD_FORM_EXPIRE_DAYS, "0"))
		assert.Nil(multipartWriter.WriteField(formkeys.UPLOAD_FORM_EXPIRE_HOURS, "0"))
		assert.Nil(multipartWriter.WriteField(formkeys.UPLOAD_FORM_EXPIRE_MINUTES, "5"))
		assert.Nil(multipartWriter.WriteField(formkeys.UPLOAD_FORM_EXPIRE_DOWNLOADS, "5"))
		assert.Nil(multipartWriter.WriteField(formkeys.UPLOAD_FORM_PASSWORD, ""))
		part, err = multipartWriter.CreateFormFile(formkeys.UPLOAD_FORM_FILE, "file.txt")
		assert.Nil(err)
		part.Write([]byte("Hello, World! this is a file."))

		assert.Nil(multipartWriter.Close())

		r, err := http.NewRequest("POST", "/api/upload", &buf)
		assert.Nil(err)

		r.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		w := httptest.NewRecorder()

		apiv := APIV1{}
		apiv.Init()
		apiv.fmap.TempDir = t.TempDir()
		apiv.file_upload(w, r)

		// read the full stream
		response := w.Result()
		defer response.Body.Close()

		assert.Equal(response.StatusCode, http.StatusSeeOther, "status code should match")
	})
}
