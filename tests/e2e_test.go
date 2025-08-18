package tests

import (
	"archivus/config"
	"archivus/internal/auth"
	"archivus/internal/db"
	"archivus/internal/models"
	"archivus/server"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	TEST_DIR = "tests/__test__dir__"
	username = "testuser"
	password = "testpassword"
)

var fnames = [2]string{
	"img1.png", "img2.png",
}
var testToken string
var testUserID string

var filesSignedUrls []string
var filePaths []string

func removeExistingTestDB() {
	dbFilePath := filepath.Join(config.BaseDir, config.TestDbFile)
	if _, err := os.Stat(dbFilePath); err == nil {
		if err := os.Remove(dbFilePath); err != nil {
			log.Println("Error removing existing test DB file:", err)
		}
	}
}

func clearDirs(usernames []string) {
	for _, username := range usernames {
		userDir := filepath.Join(config.Config.UploadsDir, username)
		if err := os.RemoveAll(userDir); err != nil {
			log.Printf("Error removing directory for user %s: %v", username, err)
		}
	}
}

func TestMain(m *testing.M) {
	removeExistingTestDB()
	db.InitTestDB()
	db.StorageDB.AutoMigrate(&models.User{}, &models.UserPreference{}, &models.Tags{}, &models.FileMetadata{}, &models.Directory{})
	if err := testSignupAndSignin(username, password); err != nil {
		log.Println("Error during signup/signin:", err)
		os.Exit(1)
	}
	code := m.Run()
	dbFilePath := filepath.Join(config.BaseDir, config.TestDbFile)
	os.Remove(dbFilePath)
	clearDirs([]string{username})
	os.Exit(code)
}

func testSignupAndSignin(username, password string) error {
	_, userId, err := auth.CreateUser(username, password, "123456", "abc@test.co.in")
	if err != nil {
		log.Println("Error creating user:", err)
		return err
	}
	var user models.User
	db.StorageDB.Where("id = ?", userId).First(&user)

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
	testToken = response["token"].(string)
	testUserID = userId
	if err != nil {
		log.Println("Error unmarshalling response:", err)
		return err
	}
	if testToken == "" || testUserID == "" {
		return fmt.Errorf("Expected non empty token and userID, got token: %s, userID: %s", testToken, testUserID)
	}
	return nil
}

func makeTestRequest(w *httptest.ResponseRecorder, req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	server.GetServer(true).Handler.ServeHTTP(w, req)
}

func decodeJson(w *httptest.ResponseRecorder) (map[string]interface{}, error) {
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %v", err)
	}
	return response, nil
}

func checkPaths(t *testing.T, expectedPaths []string) {
	basePath := filepath.Join(config.Config.UploadsDir, username)
	for _, path := range expectedPaths {
		if _, err := os.Stat(filepath.Join(basePath, path)); os.IsNotExist(err) {
			t.Errorf("Expected path %s to exist, but it does not", path)
		}
	}
}

func TestAddFolder(t *testing.T) {
	expPaths := []string{}
	payload := map[string]string{
		"folder": "folder1",
	}
	expPaths = append(expPaths, "folder1")

	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/folder/add/", bytes.NewReader(body))
	makeTestRequest(w, req, map[string]string{
		"Content-Type": "application/json", "Authorization": "Bearer " + testToken,
	})
	response, err := decodeJson(w)
	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}
	log.Println("Response:", response)
	require.Equal(t, 200, w.Code, "Expected status code 200, got %d", w.Code)

	payload = map[string]string{
		"folder": "folder2",
	}
	expPaths = append(expPaths, "folder2")
	body, _ = json.Marshal(payload)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/folder/add/", bytes.NewReader(body))
	makeTestRequest(w, req, map[string]string{
		"Content-Type": "application/json", "Authorization": "Bearer " + testToken,
	})
	response, err = decodeJson(w)
	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}
	log.Println("Response:", response)
	require.Equal(t, 200, w.Code, "Expected status code 200, got %d", w.Code)

	payload = map[string]string{
		"folder": "folder1/folder1.1",
	}
	expPaths = append(expPaths, "folder1/folder1.1")
	body, _ = json.Marshal(payload)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/folder/add/", bytes.NewReader(body))
	makeTestRequest(w, req, map[string]string{
		"Content-Type": "application/json", "Authorization": "Bearer " + testToken,
	})
	response, err = decodeJson(w)
	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}

	log.Println("Response:", response)
	require.Equal(t, 200, w.Code, "Expected status code 200, got %d", w.Code)
	checkPaths(t, expPaths)
}

func TestUploadFiles(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, f := range fnames {
		file, err := os.Open(filepath.Join(config.BaseDir, TEST_DIR, f))
		require.NoError(t, err)
		defer file.Close()

		part, err := writer.CreateFormFile("file", filepath.Base(f))
		require.NoError(t, err)
		_, err = io.Copy(part, file)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close(), "Error closing multipart writer")
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/files/upload/", &buf)
	makeTestRequest(w, req, map[string]string{
		"Content-Type":  "multipart/form-data; boundary=" + writer.Boundary(),
		"Authorization": "Bearer " + testToken,
	})
	response, err := decodeJson(w)
	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}
	log.Println("Response:", response)
	require.Equal(t, 200, w.Code, "Expected status code 200, got %d", w.Code)
	for _, f := range fnames {
		filePath := filepath.Join(config.Config.UploadsDir, username, f)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist, but it does not", filePath)
		}
	}
}

func TestListFiles(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/files/list/", nil)
	makeTestRequest(w, req, map[string]string{
		"Authorization": "Bearer " + testToken,
	})
	response, err := decodeJson(w)
	log.Println("Response:", response)
	if err != nil {
		t.Fatalf("Error unmarshalling response: %v", err)
	}
	log.Println("Response:", response)
	require.Equal(t, 200, w.Code, "Expected status code 200, got %d", w.Code)

	filesData := response["files"].([]interface{}) // this works
	var files []models.FileMetadata

	tmp, _ := json.Marshal(filesData)
	_ = json.Unmarshal(tmp, &files)

	if err != nil || len(files) != len(fnames) {
		t.Fatal("Expected files to be a non-empty slice")
	}
	for _, f := range files {
		filePaths = append(filePaths, f.FilePath)
	}
}

func TestDownloadFile(t *testing.T) {
	for _, f := range filePaths {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/files/get-signed-url/"+f, nil)
		makeTestRequest(w, req, map[string]string{
			"Authorization": "Bearer " + testToken,
		})

		if w.Code != http.StatusOK {
			t.Errorf("Expected status code 200, got %d for file %s", w.Code, f)
		}
		response, err := decodeJson(w)
		if err != nil {
			t.Fatalf("Error unmarshalling response: %v", err)
		}
		log.Println("Response:", response)
		signedUrl := response["signed_url"].(string)
		req = httptest.NewRequest("GET", "/files/download/"+signedUrl, nil)
		w = httptest.NewRecorder()
		makeTestRequest(w, req, map[string]string{
			"Authorization": "Bearer " + testToken,
		})
		if w.Code != http.StatusOK {
			t.Errorf("Expected status code 200, got %d for file %s", w.Code, f)
		}
		contentDisposition := w.Header().Get("Content-Disposition")
		if contentDisposition == "" {
			t.Errorf("Expected Content-Disposition header to be set for file %s", f)
		} else {
			log.Println("Content-Disposition:", contentDisposition)
		}
		newFileName := filepath.Join(config.BaseDir, TEST_DIR, "downloaded_"+filepath.Base(f))
		newFile, err := os.Create(newFileName)
		if err != nil {
			t.Fatalf("Error creating file %s: %v", newFileName, err)
		}
		defer newFile.Close()
		_, err = io.Copy(newFile, w.Body)
		if err != nil {
			t.Fatalf("Error writing to file %s: %v", newFileName, err)
		}
		// verify non empty file
		fileInfo, err := newFile.Stat()
		if err != nil {
			t.Fatalf("Error getting file info for %s: %v", newFileName, err)
		}
		if fileInfo.Size() == 0 {
			t.Errorf("Downloaded file %s is empty", newFileName)
		} else {
			log.Printf("Downloaded file %s successfully with size %d bytes\n", newFileName, fileInfo.Size())
		}

	}
}
