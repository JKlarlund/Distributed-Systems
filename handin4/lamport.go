package handin4

type LClock struct {
	id   int32
	Time int32
}

func InitializeLClock(id int32, currentTime int32) *LClock {
	return &LClock{
		id:   id,
		Time: currentTime,
	}
}

func (clock *LClock) Step() {
	clock.Time++
}

func (clock *LClock) SendEvent() int32 {
	clock.Step()
	return clock.Time
}

func (clock *LClock) ReceiveEvent(receivedTime int32) {
	if receivedTime > clock.Time {
		clock.Time = receivedTime
	}
	clock.Step()
}