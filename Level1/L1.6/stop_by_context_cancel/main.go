package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
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
		fmt.Println(v)
		if v == 3 {
			cancel() // ручная отмена
		}
	}
}
