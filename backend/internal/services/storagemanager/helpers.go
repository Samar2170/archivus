package storagemanager

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func createSlug(name string) string {
	slug := strings.ReplaceAll(name, " ", "-")
	slug = strings.ToLower(slug)
	slug = fmt.Sprintf("%s-%s", slug, uuid.New().String()[:8])
	return slug
}
