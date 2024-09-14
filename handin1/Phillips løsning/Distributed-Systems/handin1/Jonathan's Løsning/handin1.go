package main

import (
	"fmt"
	"sync"
	"time"
)

type P struct {
	id        int
	eat       int
	canEat    chan bool
	leftFork  chan bool
	rightFork chan bool
	wg        sync.WaitGroup
}

var wg sync.WaitGroup

var forks [5](chan bool)

func main() {
	wg.Add(5)
	for i := range forks {
		forks[i] = make(chan bool, 1)
		forks[i] <- true
	}

	for i := 0; i < 4; i++ {
		go begin(&(P{i, 0, make(chan bool, 2), forks[i], forks[i+1], sync.WaitGroup{}}))
	}
	begin(&P{4, 0, make(chan bool, 2), forks[4], forks[0], sync.WaitGroup{}})
	wg.Wait()
	fmt.Printf("All philosophers are full.")
}

func begin(p *P) {
	defer wg.Done()
	for {
		if p.eat == 3 {
			break
		}

		if tryToEat(p) {
			fmt.Printf("Philosopher %d is eating\n", p.id)
		} else {
			fmt.Printf("Philosopher %d cannot eat\n", p.id)
		}
	}
}

func tryToEat(p *P) bool {
	p.wg.Add(2)
	go getLeft(p)
	go getRight(p)
	p.wg.Wait()
	hasFirstFork := <-p.canEat
	hasSecondFork := <-p.canEat
	if hasFirstFork && hasSecondFork {
		p.eat++
		time.Sleep(3 * time.Second)
		p.leftFork <- true
		p.rightFork <- true
		return true
	} else if hasFirstFork && !hasSecondFork {
		p.leftFork <- true
	} else if !hasFirstFork && hasSecondFork {
		p.rightFork <- true
	}
	time.Sleep(2 * time.Second)
	return false
}

func getLeft(p *P) {
	defer p.wg.Done()
	select {
	case <-p.leftFork:
		p.canEat <- true
	default:
		p.canEat <- false
	}
}
func getRight(p *P) {
	defer p.wg.Done()
	select {
	case <-p.rightFork:
		p.canEat <- true
	default:
		p.canEat <- false
	}
}
