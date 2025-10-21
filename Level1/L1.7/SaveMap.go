package main

import (
	"fmt"
	"sync"
)

type SafeMap struct {
	mu sync.Mutex
	m  map[int]int
}

func NewSafeMap() *SafeMap {
	return &SafeMap{m: make(map[int]int)}
}

func (s *SafeMap) Set(k, v int) {
	s.mu.Lock()
	s.m[k] = v
	s.mu.Unlock()
}

func (s *SafeMap) Get(k int) (int, bool) {
	s.mu.Lock()
	v, ok := s.m[k]
	s.mu.Unlock()
	return v, ok
}

func (s *SafeMap) Len() int {
	s.mu.Lock()
	n := len(s.m)
	s.mu.Unlock()
	return n
}

func main() {
	sm := NewSafeMap()
	var wg sync.WaitGroup

	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				sm.Set(i, id)
			}
		}(w)
	}
	wg.Wait()

	fmt.Println("len:", sm.Len())
}
