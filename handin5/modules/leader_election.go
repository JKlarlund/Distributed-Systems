package modules

import (
	"math/rand"
	"time"
)

type Election struct {
	term          int
	votes         int
	requiredVotes int
}

/*
Initiates a leader election if a leader fails or at the beginning of the program.
Should not be called in a goroutine.
*/
func leaderElection(term int, requiredVotes int) *Election {
	//Sleep 150-300 ms to reduce leadership being contested.
	var timeout time.Duration = time.Duration(150 + rand.Intn(150))
	time.Sleep(timeout * time.Microsecond)

	return &Election{term: term + 1, votes: 0, requiredVotes: requiredVotes}
}

/*
Vote for a leader.
*/
func (e *Election) vote() {
	e.votes++
}
