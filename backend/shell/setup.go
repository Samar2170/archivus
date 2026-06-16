package shell

// import (
// 	"archivus/internal/services/auth"
// 	"fmt"
// 	"log"
// 	"os"
// )

// type Shell struct {
// 	AuthService *auth.AuthService
// }

// func S3Setup() string {
// 	filepath := getUserInput("Enter path to s3config.yaml or s3config.json: ", "")
// 	if _, err := os.Stat(filepath); os.IsNotExist(err) {
// 		fmt.Println("File does not exist")
// 		return S3Setup()
// 	}
// 	return filepath
// }

// // func (sh *Shell) NewMasterUser() (models.User, error) {
// // 	username := getUserInput("Enter username (at least 3 characters): ", "")
// // 	password := getUserInput("Enter password (at least 6 characters): ", "")
// // 	pin := getUserInput("Enter pin (exactly 6 digits): ", "")
// // 	email := getUserInput("Enter email address: ", "")

// // 	fmt.Println("Creating new user...")
// // 	fmt.Printf("Username: %s\n", username)
// // 	fmt.Printf("Password: %s\n", strings.Repeat("*", len(password)))
// // 	fmt.Printf("Pin: %s\n", strings.Repeat("*", len(pin)))
// // 	fmt.Printf("Email: %s\n", email)
// // 	fmt.Println("Please remember the password and pin")

// // 	return sh.AuthService.CreateUser(username, password, pin, email, true)
// // }

// func (sh *Shell) SetupDrive() {
// 	user, err := sh.NewMasterUser()
// 	if err != nil {
// 		log.Fatalf("Error creating master user: %v", err)
// 	}
// 	fmt.Printf("Master user created with ID: %s\n", user.ID)

// 	suggestedDriveName := fmt.Sprintf("%s's Drive", user.Username)
// 	driveName := getUserInput("Enter your organization's name or press Enter to use the suggested drive name: ", suggestedDriveName)

// 	_, err = sh.AuthService.SetupNewDrive(driveName, user.ID.String())
// 	if err != nil {
// 		log.Fatalf("Error creating drive: %v", err)
// 	}
// 	fmt.Printf("Drive '%s' created successfully\n", driveName)
// }
