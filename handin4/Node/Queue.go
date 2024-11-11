package Node

import (
	"log"
	_ "sync"
)

type Request struct {
	Timestamp int32
	NodeID    int32
}

type Queue []*Request

func (q *Queue) NewQueue() *Queue {
	q = &Queue{}
	return q
}

func (q *Queue) Add(timestamp int32, nodeID int32) {
	request := &Request{Timestamp: timestamp, NodeID: nodeID}
	*q = append(*q, request)
}

func (q *Queue) Remove(nodeID int32) {
	for i := 0; i < len(*q); i++ {
		if (*q)[i].NodeID == nodeID {
			// Remove the element at index i
			*q = append((*q)[:i], (*q)[i+1:]...)
			break
		}
	}
}

func (q *Queue) GetLowest() int32 {
	if len(*q) == 0 {
		return 0
	}
	var currentLowest = (*q)[0].Timestamp
	var currentBestNodeID = (*q)[0].NodeID
	for i := 1; i < len(*q); i++ {
		tempTimestamp := (*q)[i].Timestamp
		if currentLowest > tempTimestamp {
			currentLowest = tempTimestamp
			currentBestNodeID = (*q)[i].NodeID
		}
	}
	for i := 0; i < len(*q); i++ {
		log.Printf("LOWEST NODE IS: %v, with timestamp", (*q)[i].NodeID, (*q)[i].Timestamp)
	}

	return currentBestNodeID
}
