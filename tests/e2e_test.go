package tests

import (
	"archivus/internal/db"
	"testing"
)

func TestMain(m *testing.M) {
	db.InitTestDB()
	// code := m.Run()
	// os.Remove
}
