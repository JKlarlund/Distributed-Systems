package chat

import (
	"fmt"
	"log"
)

func HandleError(err error) bool {
	if err != nil {
		log.Println(err)
		return true
	}
	return false
}

func HandleFatalError(err error) {
	if err != nil {
		fmt.Println("Fatal error encountered")
		log.Fatalf(err.Error())
	}
}
