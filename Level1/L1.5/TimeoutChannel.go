package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan int)
	done := time.After(2 * time.Second)

	go func(out chan<- int, stop <-chan time.Time) {
		defer close(out)
		i := 0
		for {
			select {
			case <-stop:
				return
			case out <- i:
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}(ch, done)

	for v := range ch {
		fmt.Println("got:", v)
	}
}
