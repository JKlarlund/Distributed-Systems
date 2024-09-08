package main

import (
	"fmt"
	"sync"
)

var fork_1 = make(chan bool, 2)
var fork_2 = make(chan bool, 2)
var philo1CanEat = make(chan bool, 2)
var wg sync.WaitGroup

func main() {
	var wg sync.WaitGroup

	wg.Add(1)

	philo1CanEat <- false
	fork_1 <- true
	fork_2 <- true
	go philo1()

	wg.Wait()

}

func isFork_1Available() {
	var isLeftAvailable bool = <-fork_1
	var isRightAvailable bool = <-fork_2
	if isRightAvailable && isLeftAvailable {
		<-philo1CanEat
		philo1CanEat <- true
	} else {
		<-philo1CanEat
		philo1CanEat <- false
	}
	fork_1 <- isLeftAvailable
	fork_2 <- isRightAvailable
}

func philo1() {
	defer wg.Done()
	var eat int = 0
	for i := 0; i < 1; i-- {
		go isFork_1Available()
		if <-philo1CanEat {
			<-fork_1
			<-fork_2
			fork_1 <- false
			fork_2 <- false
			eat++
			fmt.Println(eat)
			<-fork_1
			<-fork_2
			fork_1 <- true
			fork_1 <- true
			philo1CanEat <- false
		} else {
			philo1CanEat <- false
		}
		if eat == 3 {
			break
		}
	}
}
