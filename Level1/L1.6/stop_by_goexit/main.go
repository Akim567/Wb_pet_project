package main

import (
	"fmt"
	"runtime"
)

func main() {
	done := make(chan struct{})

	go func() {
		defer close(done)
		fmt.Println("before Goexit")
		runtime.Goexit() // завершает только эту горутину
		// дальше код не выполнится
	}()

	<-done
	fmt.Println("main exit")
}
