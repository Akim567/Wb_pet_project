package main

import (
	"fmt"
	"sync"
)

func main() {
	in := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer fmt.Println("worker done")
		for x := range in {
			fmt.Println("work:", x)
		}
	}()

	for i := 0; i < 5; i++ {
		in <- i
	}
	close(in)
	wg.Wait()
}
