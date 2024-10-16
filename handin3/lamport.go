package chat

type LClock struct {
	id   int32
	Time int
}

func InitializeLClock(id int32, currentTime int) *LClock {
	return &LClock{
		id:   id,
		Time: currentTime,
	}
}

func (clock *LClock) Step() {
	clock.Time++
}

func (clock *LClock) sendEvent() int {
	clock.Step()
	return clock.Time
}

func (clock *LClock) receiveEvent(receivedTime int) {
	if receivedTime > clock.Time {
		clock.Time = receivedTime
	}
	clock.Step()
}
