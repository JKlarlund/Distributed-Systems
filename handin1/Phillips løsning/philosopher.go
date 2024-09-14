package main

import (
	"fmt"
	"sync"
	"time"
)

type Fork struct {
	id                int
	channel           chan bool
	resetAvailability chan bool
}

func main() {
	var philosophers = 5
	var numForks = 5
	var wg sync.WaitGroup
	var wgForks sync.WaitGroup
	doneCount := make(chan bool, philosophers)
	donePhilosophers := make(chan bool)
	doneForks := make(chan bool)
	forks := make([]*Fork, philosophers)

	// Initialize the forks
	for i := 0; i < numForks; i++ {
		forks[i] = &Fork{id: i, channel: make(chan bool, 1), resetAvailability: make(chan bool, 1)}
		forks[i].channel <- true
	}

	for i := 0; i < philosophers; i++ {
		wg.Add(1)
		wgForks.Add(1)
		var managerChannel = make(chan bool, 1)
		var resetForksChannel = make(chan bool, 1)

		go philosopher(i, resetForksChannel, managerChannel, doneCount, donePhilosophers, &wg)
		go fork(forks[i], doneForks)
		go forkManager(forks[i], forks[(i+1)%numForks], resetForksChannel, managerChannel, doneForks, &wgForks)
	}
	for i := 0; i < philosophers; i++ {
		<-doneCount
	}
	close(donePhilosophers)
	wg.Wait()
	close(doneForks)
	wgForks.Wait()

	fmt.Println("Everybody is done eating!")
}

func forkManager(leftFork *Fork, rightFork *Fork, resetForksChannel chan bool, managerChannel chan bool, endFunction <-chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-endFunction:
			return
		case <-resetForksChannel:
			leftFork.resetAvailability <- true
			rightFork.resetAvailability <- true
		case <-leftFork.channel:
			select {
			case <-rightFork.channel:
				managerChannel <- true
			default:
				leftFork.channel <- true
			}
		}
	}
}

func fork(fork *Fork, endFunction <-chan bool) {
	isAvailable := true
	for {
		select {
		case <-endFunction:
			return
		case <-fork.resetAvailability:
			isAvailable = true
		}
		if isAvailable {
			fork.channel <- true
			isAvailable = false
		}
	}
}

func philosopher(id int, resetForksChannel chan bool, managerChannel chan bool, countChannel chan bool, done <-chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	eatCount := 0
	for {
		select {
		case <-done:
			return
		case <-managerChannel:
			fmt.Println("Philosopher:", id, "is eating")
			time.Sleep(time.Second) // Eating
			eatCount++
			resetForksChannel <- true // Puts down the forks
			if eatCount == 3 {
				countChannel <- true
			}
			fmt.Println("Philosopher:", id, "is thinking")
			time.Sleep(time.Second * 2) // Thinking
		}
	}
}
