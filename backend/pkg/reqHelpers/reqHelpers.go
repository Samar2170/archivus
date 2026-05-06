package reqhelpers

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/go-playground/form"
)

func DecodeRequest(r *http.Request, dest interface{}) error {
	contentType := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(contentType, "application/json"):
		return json.NewDecoder(r.Body).Decode(dest)
	case strings.HasPrefix(contentType, "multipart/form-data"):
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return err
		}
		return mapFormData(r.MultipartForm, dest)
	default:
		return errors.New("unsupported content type: " + contentType)
	}
}

func mapFormData(mf *multipart.Form, dest interface{}) error {
	decoder := form.NewDecoder()
	return decoder.Decode(dest, mf.Value)
}
