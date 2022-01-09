package Cache

import (
	"fmt"
	"math/bits"
	"runtime"
	"sync"
	"time"
)

const (
	DEFAULT_EXPIRATION     = 10 * time.Minute
	DEFAULT_CLEAN_DURATION = 10 * time.Minute
	DEFAULT_CAP            = 1024
	DEFAULT_LRU_CLEAN_SIZE = 20
)

type Element struct {
	Value      interface{}
	Expiration int64
	LastHit    int64
}

type Cache struct {
	defaultExpiration time.Duration
	elements          map[string]Element
	capacity          int64
	size              int64
	lock              *sync.RWMutex
	cleaner           *Cleaner
}

func NewCache() (c *Cache, err error) {
	c = &Cache{
		defaultExpiration: DEFAULT_EXPIRATION,
		elements:          make(map[string]Element, DEFAULT_CAP),
		capacity:          DEFAULT_CAP,
		size:              0,
		lock:              new(sync.RWMutex),
		cleaner: &Cleaner{
			Interval: DEFAULT_CLEAN_DURATION,
			stop:     make(chan bool),
		},
	}

	go c.cleaner.Run(c)
	runtime.SetFinalizer(c, stopCleaner)
	return c, nil
}

func (c *Cache) Get(key string) (value interface{}, err error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if element, ok := c.elements[key]; ok {
		element.Expiration = time.Now().Add(DEFAULT_EXPIRATION).UnixNano()
		element.LastHit = time.Now().UnixNano()
		return element.Value, nil
	}
	return nil, fmt.Errorf("value not found for key %v", key)
}

func (c *Cache) Put(key string, value interface{}) error {
	expire := time.Now().Add(DEFAULT_EXPIRATION).UnixNano()
	lastHit := time.Now().UnixNano()

	// check cache capacity
	if c.size+1 > c.capacity {
		if err := c.removeLeastVisited(); err != nil {
			return err
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	// update the element if is found by the key provided
	if e, ok := c.elements[key]; ok {
		e.Value = value
		e.Expiration = expire
		e.LastHit = lastHit
		return nil
	}

	// or create a new element and store it into our cache
	element := Element{
		Value:      value,
		Expiration: expire,
		LastHit:    lastHit,
	}

	c.elements[key] = element
	c.size++

	return nil
}

func (c *Cache) removeLeastVisited() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var lastTime int64 = 1<<(bits.UintSize-1) - 1 // MaxInt
	lastItems := make([]string, DEFAULT_LRU_CLEAN_SIZE)
	liCount := 0
	full := false

	for k, element := range c.elements {
		if element.Expiration > time.Now().UnixNano() { // not expiring
			atime := element.LastHit
			if !full || atime < lastTime {
				lastTime = atime
				if liCount < DEFAULT_LRU_CLEAN_SIZE {
					lastItems[liCount] = k
					liCount++
				} else {
					lastItems[0] = k
					liCount = 1
					full = true
				}
			}
		}
	}

	for i := 0; i < len(lastItems) && lastItems[i] != ""; i++ {
		lastName := lastItems[i]
		delete(c.elements, lastName)
	}
	return nil
}

func (c *Cache) Delete(key string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.elements[key]; ok {
		delete(c.elements, key)
		return nil
	}
	return fmt.Errorf("impossible remove item with key %v", key)
}

func (c *Cache) Flush() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.size > 0 {
		c.elements = make(map[string]Element, DEFAULT_CAP)
	}
	return nil
}

func (c *Cache) RemoveExpired() {
	c.lock.Lock()
	defer c.lock.Unlock()

	for key, element := range c.elements {
		if element.Expiration > 0 && time.Now().UnixNano() > element.Expiration {
			delete(c.elements, key)
		}
	}
}

type Cleaner struct {
	Interval time.Duration
	stop     chan bool
}

func (cleaner *Cleaner) Run(c *Cache) {
	ticker := time.NewTicker(cleaner.Interval)
	for {
		select {
		case <-ticker.C:
			c.RemoveExpired()
		case <-cleaner.stop:
			ticker.Stop()
			return
		}
	}
}

func stopCleaner(c *Cache) error {
	c.cleaner.stop <- true
	return nil
}
