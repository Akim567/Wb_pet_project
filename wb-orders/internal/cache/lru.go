package cache

import (
	"container/list"
	"sync"
	"time"

	"wb-orders/internal/models"
)

// K — ключ заказа, string (id).
type K = string

type V = models.Order

type entry struct {
	key   K
	value V
	atime time.Time // последний доступ — пригодится для метрик/отладки
}

type LRU struct {
	mu       sync.RWMutex
	capacity int
	ll       *list.List          // список от свежего (Front) к старому (Back)
	index    map[K]*list.Element // key -> элемент списка (*entry в e.Value)
	hits     uint64
	misses   uint64
}

func NewLRU(capacity int) *LRU {
	if capacity <= 0 {
		capacity = 1
	}
	return &LRU{
		capacity: capacity,
		ll:       list.New(),
		index:    make(map[K]*list.Element, capacity),
	}
}

func (c *LRU) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.index[key]; ok {
		en := el.Value.(*entry)
		en.atime = time.Now()
		c.ll.MoveToFront(el)
		c.hits++
		return en.value, true
	}
	c.misses++
	var zero V
	return zero, false
}

func (c *LRU) Set(key K, val V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.index[key]; ok {
		en := el.Value.(*entry)
		en.value = val
		en.atime = time.Now()
		c.ll.MoveToFront(el)
		return
	}

	// вставляем новый
	en := &entry{key: key, value: val, atime: time.Now()}
	el := c.ll.PushFront(en)
	c.index[key] = el

	// вытеснение
	if c.ll.Len() > c.capacity {
		tail := c.ll.Back()
		if tail != nil {
			ev := tail.Value.(*entry)
			delete(c.index, ev.key)
			c.ll.Remove(tail)
		}
	}
}

func (c *LRU) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ll.Len()
}

func (c *LRU) Stats() (hits, misses uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}
