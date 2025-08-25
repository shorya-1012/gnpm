package commandhandler

import (
	"fmt"

	"github.com/shorya-1012/gnpm/internal/installer"
)

type CommandHandler struct {
	command  string
	argument string
}

func (ch *CommandHandler) ParseCommand(args []string) {
	command := args[1]
	packageInfo := args[2]

	ch.command = command
	ch.argument = packageInfo
}

func (ch *CommandHandler) DebugDisplay() {
	fmt.Println(ch.command)
	fmt.Println(ch.argument)
}

func (ch *CommandHandler) Execute() {
	switch ch.command {
	case "i", "install":
		fmt.Println("Installing ... ")
		installer := installer.NewInstaller()
		installer.HandleInstall(ch.argument)
	default:
		fmt.Println("Command not found")
	}
}
