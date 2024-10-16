package chat

import (
	"fmt"
	"log"
)

func HandleError(err error) bool {
	if err != nil {
		log.Println(err)
		fmt.Println(err.Error())
		return true
	}
	return false
}

func HandleFatalError(err error) {
	if err != nil {
		fmt.Println("Fatal error encountered")
		fmt.Println(err.Error())
		log.Fatalf(err.Error())
	}
}
