package commandhandler

import (
	"fmt"
	"github.com/shorya-1012/gnpm/internal/parser"
)

type CommandHandler struct {
	command        string
	packageName    string
	packageVersion string
}

func (ch *CommandHandler) ParseCommand(args []string) {
	command := args[1]
	packageInfo := args[2]

	ch.command = command
	ch.packageName, ch.packageVersion = parser.ParseVersion(packageInfo)
}

func (ch *CommandHandler) DebugDisplay() {
	fmt.Println(ch.packageName)
	fmt.Println(ch.packageVersion)
	fmt.Println(ch.command)
}

func (ch *CommandHandler) Exectue() {
	switch ch.command {
	case "install":
		fmt.Println("Installing")
	default:
		fmt.Println("Command not found")
	}
}
