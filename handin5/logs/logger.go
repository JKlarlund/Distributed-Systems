package logs

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

func WriteToLog(logger *log.Logger, message string, timestamp int32, id int32) {
	if id == 0 {
		logger.Printf("[SERVER] %s at Lamport time: %d", message, timestamp)
	} else {
		logger.Printf("[User %d] %s at Lamport time: %d", id, message, timestamp)
	}
}

func InitLoggerWithUniqueID(filename string) *log.Logger {
	filename = filename + strconv.Itoa(os.Getpid())
	logFile, err := os.OpenFile(filename+".log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		fmt.Println(err)
	}

	return log.New(logFile, "", log.Ldate|log.Ltime)
}

func ClearClientLogs() {
	matches, _ := filepath.Glob("Client-*.log")
	fmt.Println(len(matches))
	for _, match := range matches {

		err := os.Remove(match)
		if err != nil {
			fmt.Printf("%v", err)
		}
	}
}
