package main

import (
	"fmt"
	"sync"
)

func main() {
	nums := []int{2, 4, 6, 8, 10}

	fmt.Println("input", nums)

	// Создаем группу ожидания
	var wg sync.WaitGroup

	for _, n := range nums {
		wg.Add(1)

		go func(x int) {
			defer wg.Done()
			fmt.Println(x * x)
		}(n)
	}

	wg.Wait()
}
