package shell

import (
	"archivus/internal/models"
	"archivus/internal/store"
	"fmt"
	"log"
	"strings"
)

type Shell struct {
	Store *store.Store
}

func (sh *Shell) NewMasterUser() (models.User, error) {
	username := getUserInput("Enter username (at least 3 characters): ", "")
	password := getUserInput("Enter password (at least 6 characters): ", "")
	pin := getUserInput("Enter pin (exactly 6 digits): ", "")
	email := getUserInput("Enter email address: ", "")

	fmt.Println("Creating new user...")
	fmt.Println("Params:")
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Password: %s\n", strings.Repeat("*", len(password)))
	fmt.Printf("Pin: %s\n", strings.Repeat("*", len(pin)))
	fmt.Printf("Email: %s\n", email)
	fmt.Printf("Please remember the password and pin")

	user, err := sh.Store.CreateUser(username, password, pin, email, true)
	if err != nil {
		return models.User{}, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

func (sh *Shell) SetupDrive() {
	user, err := sh.NewMasterUser()
	if err != nil {
		log.Fatalf("Error creating master user: %v", err)
	}
	fmt.Printf("Master user created with ID: %s\n", user.ID)
	suggestedDriveName := fmt.Sprintf("%s's Drive", user.Username)
	driveName := getUserInput("Enter your organization's name or press Enter to use the suggested drive name: ", suggestedDriveName)

	err = sh.Store.SetupNewDrive(driveName, user.ID.String())
	if err != nil {
		log.Fatalf("Error creating drive: %v", err)
	}
	fmt.Printf("Drive '%s' created successfully\n", driveName)
}
