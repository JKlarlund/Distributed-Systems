package chat

type LClock struct {
	id   int32
	time int
}

func InitializeLClock(id int32, currentTime int) *LClock {
	return &LClock{
		id:   id,
		time: currentTime,
	}
}

func (clock *LClock) step() {
	clock.time++
}

func (clock *LClock) sendEvent() int {
	clock.step()
	return clock.time
}

func (clock *LClock) receiveEvent(receivedTime int) {
	if receivedTime > clock.time {
		clock.time = receivedTime
	}
	clock.step()
}
