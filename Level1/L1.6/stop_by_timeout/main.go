package main

import (
	"fmt"
	"time"
)

func main() {
	data := make(chan int)
	timeout := time.After(1500 * time.Millisecond)

	go func() {
		defer close(data)
		i := 0
		for {
			select {
			case <-timeout:
				return
			case data <- i:
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()

	for v := range data {
		fmt.Println("got:", v)
	}
}
