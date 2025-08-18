package main

import (
	"archivus/migrator"
	"archivus/server"
	"archivus/shell"
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

func getUserInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	value, _ := reader.ReadString('\n')
	value = strings.ReplaceAll(value, "\n", "")
	return value
}

var Commands = map[string][2]string{
	"setup":    {"setup", "Initialize the application with necessary configurations."},
	"sync":     {"sync", "Synchronize the application with the existing data."},
	"start":    {"start", "Start the application server."},
	"new-user": {"new-user", "Create a new user in the application."},
}

func newUser() {
	username := getUserInput("Enter username (at least 3 characters): ")
	password := getUserInput("Enter password (at least 6 characters): ")
	pin := getUserInput("Enter pin (exactly 6 digits): ")
	email := getUserInput("Enter email address: ")
	fmt.Println("Creating new user...")
	fmt.Println("Params:")
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Password: %s\n", strings.Repeat("*", len(password)))
	fmt.Printf("Pin: %s\n", strings.Repeat("*", len(pin)))
	fmt.Printf("Email: %s\n", email)

	err := shell.CreateNewUser(username, password, pin, email)
	if err != nil {
		log.Fatalf("Error creating user: %v", err)
	}

}

func Run(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "setup":
			// This command is used to set up the application.
			log.Println("Setting up the application...")
		case "migrate":
			log.Println("Migrating the database...")
			migrator.Migrate()
		case "sync":
			log.Println("Synchronizing the application with existing data...")
		case "start":
			log.Println("Starting the application server...")
			server.RunServer()
		case "new-user":
			shell.Setup()
			newUser()
		}
	} else {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			server.RunServer()
			wg.Done()
		}()
		// wg.Add(1)
		// go func() {
		// 	time.Sleep(2 * time.Second) // Wait for server to start
		// 	startCronServer()
		// 	wg.Done()
		// }()
		wg.Wait()
	}
}

func main() {
	// This is the main entry point of the application.
	// It can be used to initialize and run the service.
	// Currently, it serves as a placeholder for future logic.
	args := os.Args[1:]
	Run(args)

}
