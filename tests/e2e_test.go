package tests

import (
	"archivus/config"
	"archivus/internal/auth"
	"archivus/internal/db"
	"archivus/internal/models"
	"archivus/server"
	"bytes"
	"encoding/json"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	db.InitTestDB()
	db.StorageDB.AutoMigrate(&models.User{}, &models.UserPreference{}, &models.Tags{}, &models.FileMetadata{}, &models.Directory{})
	code := m.Run()
	dbFilePath := filepath.Join(config.BaseDir, config.TestDbFile)
	os.Remove(dbFilePath)
	os.Exit(code)
}

func TestSignupAndSignin(t *testing.T) {
	ak, userId, err := auth.CreateUser("testuser", "testpassword", "123456", "abc@test.co.in")
	require.NoError(t, err)
	require.NotEmpty(t, ak)
	var user models.User
	db.StorageDB.Where("id = ?", userId).First(&user)
	require.Equal(t, "testuser", user.Username)

	server := server.GetServer(true)

	payload := map[string]string{
		"username": "testuser",
		"password": "testpassword",
	}

	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	server.Handler.ServeHTTP(w, req)
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	log.Println("Response:", response)
	require.Equal(t, 200, w.Code)
	require.NoError(t, err)
	require.NotEmpty(t, response["token"])
	require.Equal(t, userId, response["user_id"])

}
