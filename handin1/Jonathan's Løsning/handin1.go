package main

import (
	"fmt"
	"sync"
	"time"
)

var forks [5]chan bool

var isForkAvailable [5]chan bool

var wg sync.WaitGroup

//The system does not deadlock as philosophers tries to pick up forks, and--
//a) picks up both forks, eats, and passes them back through the forks channel, or--
//b) picks up their left fork and passes it back through the fork channel.
//The fork goroutines wait for the fork to be sent back before making it available once again.
//A WaitGroup ensures that all philosophers have eaten three times before terminating the program.

func main() {
	for i := range 5 {
		forks[i] = make(chan bool, 1)
		forks[i] <- true
		isForkAvailable[i] = make(chan bool, 1)
	}
	for i := range 5 {
		wg.Add(1)
		go fork(i)
		go philosopher(i)
	}

	wg.Wait()
	fmt.Printf("All philosophers have finished eating!\n")
}

func fork(id int) {
	for {
		<-forks[id]
		isForkAvailable[id] <- true
	}
}

func philosopher(id int) {
	defer wg.Done()
	var eat int
	leftfork := id
	rightfork := (id + 1) % 5

	for {
		select {
		case <-isForkAvailable[leftfork]:
			select {
			case <-isForkAvailable[rightfork]:
				eat++
				eating(id)
				time.Sleep(1 * time.Second)

				forks[leftfork] <- true
				forks[rightfork] <- true
				if eat == 3 {
					fmt.Printf("%d is done eating...\n", id)
					return
				}
			default:
				thinking(id)
				forks[leftfork] <- true

			}
		default:
			thinking(id)
		}
	}
}

func eating(id int) {
	fmt.Printf("%d is eating...\n", id)
	time.Sleep(1 * time.Second)
}

func thinking(id int) {
	fmt.Printf("%d is thinking...\n", id)
	time.Sleep(1 * time.Second)
}
