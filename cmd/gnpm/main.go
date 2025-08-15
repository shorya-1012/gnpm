package main

import (
	"fmt"
	"os"

	commandhandler "github.com/shorya-1012/gnpm/internal/command_handler"
	// "github.com/shorya-1012/gnpm/internal/profile"
)

func main() {
	// profile.Setup()

	if len(os.Args) < 3 {
		fmt.Println("Usage : \ngnpm install <package>")
		return
	}

	var commandHandler commandhandler.CommandHandler
	commandHandler.ParseCommand(os.Args)

	commandHandler.Execute()
}
