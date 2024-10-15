package main

type LClock struct {
	id   int
	time int
}

func initializeLClock(id int, currentTime int) *LClock {
	return &LClock{
		id:   id,
		time: currentTime,
	}
}

func (clock *LClock) step() {
	clock.time++
}
