package chat

import (
	"log"
	"os"
)

func WriteToLog(logger *log.Logger, message string, timestamp int32, id int32) {
	if id == 0 {
		logger.Printf("[SERVER] %s at Lamport time: %d", message, timestamp)
	} else {
		logger.Printf("[User %d] %s at Lamport time: %d", id, message, timestamp)
	}

}

func InitLogger(filename string) *log.Logger {
	clientLog, err := os.OpenFile(filename+".log", os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
	}

	return log.New(clientLog, "", log.Ldate|log.Ltime)
}
