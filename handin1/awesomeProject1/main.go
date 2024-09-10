package main

import (
	"fmt"
	"math"
)

var orange, apple, pineapple int
var orangeShare, appleShare, pineappleShare int
var orangeResult, appleResult, pineappleResult float32

func main() {
	fmt.Scan(&orange, &apple, &pineapple, &orangeShare, &appleShare, &pineappleShare)
	var orangeRatio, appleRatio, pineappleRatio = float64(orange / orangeShare), float64(apple / appleShare), float64(pineapple / pineappleShare)
	var minRatio = math.Min(orangeRatio, math.Min(appleRatio, pineappleRatio))
	switch minRatio {
	case orangeRatio:
		fmt.Println("Orange")
	case appleRatio:
		fmt.Println("Apple")
	case pineappleRatio:
		fmt.Println("Pineapple")
	}
}
