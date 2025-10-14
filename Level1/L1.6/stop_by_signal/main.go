package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	data := make(chan int)
	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	// переводим системный сигнал в наш stop
	go func() {
		<-sigs
		close(stop)
	}()

	go func() {
		defer close(data)
		i := 0
		for {
			select {
			case <-stop:
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
