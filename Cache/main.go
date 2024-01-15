package main

import (
	"fmt"
	"sync"
)

type Storage interface {
	put(key, value interface{}) error
	get(key interface{}) (interface{}, error)
	remove(key interface{})
	isFull() bool
}

type StorageFullException struct {
	message string
}

func (e *StorageFullException) Error() string {
	return e.message
}

type NotFoundException struct {
	message string
}

func (e *NotFoundException) Error() string {
	return e.message
}

type EvictionPolicy interface {
	accessedKey(key interface{})
	evictKey() interface{}
}

type LinkedListNode struct {
	element interface{}
	prev    *LinkedListNode
	next    *LinkedListNode
}

type DoubleLinkedList struct {
	head *LinkedListNode
	tail *LinkedListNode
}

func newDoubleLinkedList() *DoubleLinkedList {
	head := &LinkedListNode{}
	tail := &LinkedListNode{}
	head.next = tail
	tail.prev = head
	return &DoubleLinkedList{head: head, tail: tail}
}

func (dll *DoubleLinkedList) remove(node *LinkedListNode) {
	if node != nil {
		node.prev.next = node.next
		node.next.prev = node.prev
	}
}

func (dll *DoubleLinkedList) getNodeAtHead() *LinkedListNode {
	if !dll.isElementPresent() {
		return nil
	}
	return dll.head.next
}

func (dll *DoubleLinkedList) isElementPresent() bool {
	return dll.head.next != dll.tail
}

func (dll *DoubleLinkedList) addToTail(node *LinkedListNode) {
	node.prev = dll.tail.prev
	node.next = dll.tail
	dll.tail.prev.next = node
	dll.tail.prev = node
}

type CacheProvider struct {
	storage       Storage
	evictionPolicy EvictionPolicy
}

func NewCacheProvider(storage Storage, evictionPolicy EvictionPolicy) *CacheProvider {
	return &CacheProvider{storage: storage, evictionPolicy: evictionPolicy}
}

func (cp *CacheProvider) put(key, value interface{}) {
	err := cp.storage.put(key, value)
	if err == nil {
		cp.evictionPolicy.accessedKey(key)
	} else if _, ok := err.(*StorageFullException); ok {
		evictKey := cp.evictionPolicy.evictKey()
		if evictKey == nil {
			panic("Unexpected state..")
		}
		cp.storage.remove(evictKey)
	}
}

func (cp *CacheProvider) get(key interface{}) interface{} {
	value, err := cp.storage.get(key)
	if err == nil {
		cp.evictionPolicy.accessedKey(key)
		return value
	} else if _, ok := err.(*NotFoundException); ok {
		return nil
	}
	return nil
}

type InMemoryStorage struct {
	storage  map[interface{}]interface{}
	capacity int
	mu       sync.Mutex
}

func NewInMemoryStorage(capacity int) *InMemoryStorage {
	return &InMemoryStorage{
		storage:  make(map[interface{}]interface{}),
		capacity: capacity,
	}
}

func (ims *InMemoryStorage) put(key, value interface{}) error {
	ims.mu.Lock()
	defer ims.mu.Unlock()

	ims.storage[key] = value
	if ims.isFull() {
		return &StorageFullException{"Storage is full"}
	}
	return nil
}

func (ims *InMemoryStorage) get(key interface{}) (interface{}, error) {
	ims.mu.Lock()
	defer ims.mu.Unlock()

	if value, ok := ims.storage[key]; ok {
		return value, nil
	}
	return nil, &NotFoundException{fmt.Sprintf("%v not found", key)}
}

func (ims *InMemoryStorage) remove(key interface{}) {
	ims.mu.Lock()
	defer ims.mu.Unlock()

	delete(ims.storage, key)
}

func (ims *InMemoryStorage) isFull() bool {
	return len(ims.storage) >= ims.capacity
}

type LRUEvictionPolicy struct {
	doubleLinkedList *DoubleLinkedList
	mapper           map[interface{}]*LinkedListNode
	mu               sync.Mutex
}

func NewLRUEvictionPolicy() *LRUEvictionPolicy {
	return &LRUEvictionPolicy{
		doubleLinkedList: newDoubleLinkedList(),
		mapper:           make(map[interface{}]*LinkedListNode),
	}
}

func (ep *LRUEvictionPolicy) accessedKey(key interface{}) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	var node *LinkedListNode
	if existingNode, ok := ep.mapper[key]; ok {
		node = existingNode
		ep.doubleLinkedList.remove(node)
	} else {
		node = &LinkedListNode{element: key}
		ep.mapper[key] = node
	}
	ep.doubleLinkedList.addToTail(node)
}

func (ep *LRUEvictionPolicy) evictKey() interface{} {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	lruNode := ep.doubleLinkedList.getNodeAtHead()
	if lruNode == nil {
		return nil
	}
	ep.doubleLinkedList.remove(lruNode)
	delete(ep.mapper, lruNode.element)
	return lruNode.element
}

func main() {
	// Example usage
	capacity := 3
	storage := NewInMemoryStorage(capacity)
	evictionPolicy := NewLRUEvictionPolicy()
	cacheProvider := NewCacheProvider(storage, evictionPolicy)

	cacheProvider.put("1", "One")
	cacheProvider.put("2", "Two")
	cacheProvider.put("3", "Three")
	fmt.Println(cacheProvider.get("1")) // Output: One
	cacheProvider.put("4", "Four")       // Eviction of "2"
	fmt.Println(cacheProvider.get("2")) // Output: nil
}

