package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type Item struct {
	i int
}

func createItem() Item {
	return Item{i: rand.Intn(10)}
}

type MyBlockingQueue struct {
	queue    []Item
	lock     sync.Mutex
	notFull  *sync.Cond
	notEmpty *sync.Cond
	maxSize  int
}

func NewMyBlockingQueue(size int) *MyBlockingQueue {
	return &MyBlockingQueue{
		queue:    make([]Item, 0),
		lock:     sync.Mutex{},
		notFull:  sync.NewCond(&sync.Mutex{}),
		notEmpty: sync.NewCond(&sync.Mutex{}),
		maxSize:  size,
	}
}

func (bq *MyBlockingQueue) take() Item {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	for len(bq.queue) == 0 {
		bq.notEmpty.Wait()
	}

	item := bq.queue[0]
	bq.queue = bq.queue[1:]
	bq.notFull.Signal()

	return item
}

func (bq *MyBlockingQueue) put(item Item) {
	bq.lock.Lock()
	defer bq.lock.Unlock()

	for len(bq.queue) == bq.maxSize {
		bq.notFull.Wait()
	}

	bq.queue = append(bq.queue, item)
	bq.notEmpty.Signal()
}

func producer(bq *MyBlockingQueue, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		fmt.Println("producer")
		item := createItem()
		fmt.Printf("putting item %d\n", item.i)
		bq.put(item)
		time.Sleep(1 * time.Second)
	}
}

func consumer(bq *MyBlockingQueue, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		fmt.Println("consumer")
		item := bq.take()
		fmt.Printf("consume %d\n", item.i)
		time.Sleep(2 * time.Second)
	}
}

func main() {
	// Using your own fixed-sized queue with locks instead of a blocking queue
	bq := NewMyBlockingQueue(2)

	var wg sync.WaitGroup

	// Start producers
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go producer(bq, &wg)
	}

	// Start consumers
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go consumer(bq, &wg)
	}

	wg.Wait()
}

