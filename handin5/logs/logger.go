package logs

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func WriteToLog(logger *log.Logger, message string, timestamp int32, id int32) {
	if id == 0 {
		logger.Printf("[SERVER] %s at Lamport time: %d", message, timestamp)
	} else {
		logger.Printf("[User %d] %s at Lamport time: %d", id, message, timestamp)
	}
}

func WriteToServerLog(logger *log.Logger, message string, timestamp int32) {
	logger.Printf("[SERVER] %s at Lamport time: %d", message, timestamp)
}

func InitLogger(filename string) *log.Logger {
	clientLog, err := os.OpenFile("logs/"+filename+".log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		fmt.Println(err)
	}

	return log.New(clientLog, "", log.Ldate|log.Ltime)
}

// Gotta finish this.
func RenameLogger(logFile **log.Logger, oldName string, newName string) {
	closeFile := log.New(os.Stdout, "", log.Ldate|log.Ltime)
	*logFile = closeFile
	err := os.Rename("logs/"+oldName+".log", "logs/"+newName+".log")
	if err != nil {
		fmt.Println(err)
	}
}

func InitServerLogger(isPrimary bool) *log.Logger {
	var filename string
	if isPrimary {
		filename = "Server - Primary"
	} else {
		filename = "Server - Backup"
	}
	fmt.Println(filename)
	clientLog, err := os.OpenFile("logs/"+filename+".log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		fmt.Println(err)
	}

	return log.New(clientLog, "", log.Ldate|log.Ltime)
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
