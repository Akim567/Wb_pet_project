package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan int)
	
	go func(ch chan<- int) {
		defer close(ch)
		i := 0
		for {
			select {
			case ch <- i:
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}(ch)

	for x := range ch {
		fmt.Println(x)
		if x == 3 {
			close(ch)
		}
	}
}
