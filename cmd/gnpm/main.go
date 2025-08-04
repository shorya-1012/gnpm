package main

import (
	"fmt"
	"os"

	commandhandler "github.com/shorya-1012/gnpm/internal/command_handler"
)

func main() {

	if len(os.Args) < 3 {
		fmt.Println("Usage : \ngnpm install <package>")
		return
	}

	var commandHandler commandhandler.CommandHandler
	commandHandler.ParseCommand(os.Args)

	commandHandler.DebugDisplay()

	// get the command line arg

	// requestUrl := fmt.Sprintf("%s/%s", constants.RegistryBaseURL, "lodash")
	//
	// response, err := http.Get(requestUrl)
	// if err != nil {
	// 	fmt.Println("Unable to send request to registry")
	// 	fmt.Println(err)
	// 	return
	// }
	// defer response.Body.Close()
	//
	// var pakageData models.PackageData
	//
	// if err := json.NewDecoder(response.Body).Decode(&pakageData); err != nil {
	// 	fmt.Println("Error decoding json")
	// 	fmt.Println(err)
	// }
	//
	// fmt.Printf("%+v", pakageData.DistTags.Latest)
}
