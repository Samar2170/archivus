package reqhelpers

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/go-playground/form"
)

func DecodeRequest(r *http.Request, dest interface{}) error {
	contentType := r.Header.Get("Content-Type")
	switch {
	case contentType == "", strings.HasPrefix(contentType, "application/json"):
		if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("request body is empty")
			}
			return err
		}
		return nil
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
