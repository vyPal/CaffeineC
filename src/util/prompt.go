package util

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func PromptString(prompt string, def string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (%s): ", prompt, def)

	response, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	response = strings.TrimSpace(response)

	if response == "" {
		return def
	}

	return response
}

func PromptYN(prompt string, def bool) bool {
	reader := bufio.NewReader(os.Stdin)

	if def {
		fmt.Printf("%s (Y/n): ", prompt)
	} else {
		fmt.Printf("%s (y/N): ", prompt)
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	response = strings.TrimSpace(response)

	if response == "" {
		return def
	}

	return strings.ToLower(response) == "y"
}
