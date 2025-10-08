package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("usage:", os.Args[0], "<workers>")
		return
	}
	n, err := strconv.Atoi(os.Args[1])
	if err != nil || n <= 0 {
		fmt.Println("workers must be a positive integer")
		return
	}
	counts := make([]int64, n)
	ch := make(chan int, n*2)

	var wg sync.WaitGroup
	wg.Add(n)

	for id := 1; id <= n; id++ {
		go worker(id, ch, &wg, counts)
	}

	// При получении сигнала прерывания (Ctrl+C или SIGTERM) главная горутина закрывает общий канал, из которого читают все воркеры.
	//Закрытие канала автоматически завершает циклы for range ch во всех горутинах, после чего WaitGroup гарантирует их корректное завершение перед выходом из программы.

	stop := make(chan os.Signal, 1)                    // канал для сигналов ОС
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM) // подписываемся на Ctrl+C (SIGINT) и SIGTERM
	ticker := time.NewTicker(100 * time.Millisecond)   // тикер задаёт ритм записи в канал
	defer ticker.Stop()

	i := 0

	for {
		select {
		case <-stop:
			// получен Ctrl+C или SIGTERM → делаем graceful shutdown:
			close(ch) // закрываем канал — все воркеры завершат range
			wg.Wait() // ждём, пока все воркеры закончат работу
			fmt.Println("summary per worker:")
			for i := 0; i < n; i++ {
				fmt.Printf("  reader %d handled %d items\n", i+1, counts[i])
			}
			signal.Stop(stop) // отписываемся от сигналов
			return            // выходим из main
		case <-ticker.C:
			// очередной тик → отправляем данные воркерам
			ch <- i
			i++
		}
	}
}

func worker(id int, ch <-chan int, wg *sync.WaitGroup, counts []int64) {
	defer wg.Done()
	fmt.Println("Starting reader", id)
	for x := range ch {
		fmt.Println("reader", id, "got", x)
		atomic.AddInt64(&counts[id-1], 1)
		time.Sleep(120 * time.Millisecond)
	}
	fmt.Println("reader", id, "done")
}
