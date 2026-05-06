package shell

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getUserInput(prompt, defaultValue string) string {
	fmt.Print(prompt)

	// Check if stdin is a terminal
	fi, err := os.Stdin.Stat()
	if err != nil {
		fmt.Println("Error accessing stdin:", err)
		return defaultValue
	}

	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// Not a terminal (e.g., stdin redirected), return default
		fmt.Println("[No terminal detected, using default value]")
		return defaultValue
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input:", err)
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	return input
}

func SudoCheck() error {
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}
