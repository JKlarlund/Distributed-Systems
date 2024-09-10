package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Fork struct {
	id      int
	channel chan bool
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

	for i := 0; i < numForks; i++ {
		forks[i] = &Fork{id: i, channel: make(chan bool, 1)}
		forks[i].channel <- true
	}

	for i := 0; i < philosophers; i++ {
		wg.Add(1)
		wgForks.Add(1)
		var managerChannel = make(chan bool, 1)

		go philosopher(i, forks[i], forks[(i+1)%numForks], managerChannel, doneCount, donePhilosophers, &wg)
		go forkManager(forks[i], forks[(i+1)%numForks], managerChannel, doneForks, &wgForks)
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

func forkManager(leftFork *Fork, rightFork *Fork, managerChannel chan bool, doneFork <-chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-doneFork:
			return
		case <-leftFork.channel:
			select {
			case <-rightFork.channel:
				managerChannel <- true
			default:
				leftFork.channel <- true
				// After the fork is put down, the philosopher will wait one second to try again
				time.Sleep(time.Second * 1)
			}
		}
	}

}

func philosopher(id int, leftFork *Fork, rightFork *Fork, managerChannel chan bool, countChannel chan bool, done <-chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	eatCount := 0
	for {
		select {
		case <-done:
			return
		case <-managerChannel:
			fmt.Println("Philosopher:", id, "is eating")
			time.Sleep(time.Second * time.Duration(rand.Intn(2))) // Eating is taking a random amount of time between 1-2 seconds
			eatCount++
			leftFork.channel <- true
			rightFork.channel <- true
			if eatCount == 3 {
				countChannel <- true
			}
			fmt.Println("Philosopher:", id, "is thinking")
			time.Sleep(time.Second * 3) // Thinking
		}

	}
}
