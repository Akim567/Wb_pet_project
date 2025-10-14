package main

import (
	"fmt"
	"time"
)

func main() {
	data := make(chan int)
	stop := make(chan struct{})
	done := make(chan struct{})

	go func(out chan<- int, stop <-chan struct{}, done chan<- struct{}) {
		defer close(done)
		defer close(out) // закрывает тот, кто пишет
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}

			select {
			case <-stop:
				return
			case out <- i:
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}(data, stop, done)

	for x := range data {
		fmt.Println(x)
		if x == 3 {
			close(stop)
			<-done
		}
	}
	fmt.Println("finished")
}
