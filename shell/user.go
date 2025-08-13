package shell

import (
	"archivus/internal/auth"
	"archivus/internal/service"
	"fmt"
)

func Setup() {
	service.Setup(false)
}

func CreateNewUser(username, password, pin, email string) error {
	if len(username) < 3 {
		return fmt.Errorf("username must be at least 3 characters long")
	}
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters long")
	}
	if len(pin) != 6 {
		return fmt.Errorf("pin must be exactly 6 digits long")
	}
	ak, _, err := auth.CreateUser(username, password, pin, email)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Println("User created successfully!")
	fmt.Println("Save this API key securely:")
	fmt.Println(ak)
	return nil
}
