package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	data := make(chan int)
	go func(ctx context.Context) {
		defer close(data)
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case data <- i:
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}(ctx)

	for v := range data {
		fmt.Println("v:", v)
	}
}
