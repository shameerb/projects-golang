package main

import (
	"container/list"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// Storage interface
type Storage interface {
	Put(key interface{}, value interface{})
	Get(key interface{}) (interface{}, error)
	Remove(key interface{})
}

// EvictionPolicy interface
type EvictionPolicy interface {
	KeyAccessed(key interface{})
	EvictKey() interface{}
}

// MapStorage struct
type MapStorage struct {
	capacity int
	storage  map[interface{}]interface{}
	mu       sync.Mutex
}

// NewMapStorage creates a new MapStorage instance
func NewMapStorage(capacity int) *MapStorage {
	return &MapStorage{
		capacity: capacity,
		storage:  make(map[interface{}]interface{}),
	}
}

// Put method for MapStorage
func (ms *MapStorage) Put(key interface{}, value interface{}) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.storage[key]; !exists && len(ms.storage) == ms.capacity {
		panic(errors.New("Storage is full. Cannot add key"))
	}

	ms.storage[key] = value
}

// Get method for MapStorage
func (ms *MapStorage) Get(key interface{}) (interface{}, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if val, exists := ms.storage[key]; exists {
		return val, nil
	}
	return nil, errors.New("Cannot find data for key")
}

// Remove method for MapStorage
func (ms *MapStorage) Remove(key interface{}) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.storage[key]; exists {
		delete(ms.storage, key)
	}
}

// LRUEvictionPolicy struct
type LRUEvictionPolicy struct {
	dll *list.List
	mu  sync.Mutex
}

// NewLRUEvictionPolicy creates a new LRUEvictionPolicy instance
func NewLRUEvictionPolicy() *LRUEvictionPolicy {
	return &LRUEvictionPolicy{
		dll: list.New(),
	}
}

// KeyAccessed method for LRUEvictionPolicy
func (lru *LRUEvictionPolicy) KeyAccessed(key interface{}) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	for e := lru.dll.Front(); e != nil; e = e.Next() {
		if e.Value == key {
			lru.dll.MoveToBack(e)
			return
		}
	}

	lru.dll.PushBack(key)
}

// EvictKey method for LRUEvictionPolicy
func (lru *LRUEvictionPolicy) EvictKey() interface{} {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if lru.dll.Len() == 0 {
		return nil
	}

	e := lru.dll.Front()
	key := e.Value
	lru.dll.Remove(e)

	return key
}

// LRUEvictionPolicyWithCustomDataStructure struct
type LRUEvictionPolicyWithCustomDataStructure struct {
	mapper map[interface{}]*list.Element
	dll    *DoubleLinkedList
	mu     sync.Mutex
}

// NewLRUEvictionPolicyWithCustomDataStructure creates a new LRUEvictionPolicyWithCustomDataStructure instance
func NewLRUEvictionPolicyWithCustomDataStructure() *LRUEvictionPolicyWithCustomDataStructure {
	return &LRUEvictionPolicyWithCustomDataStructure{
		mapper: make(map[interface{}]*list.Element),
		dll:    NewDoubleLinkedList(),
	}
}

// KeyAccessed method for LRUEvictionPolicyWithCustomDataStructure
func (lru *LRUEvictionPolicyWithCustomDataStructure) KeyAccessed(key interface{}) {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if node, exists := lru.mapper[key]; exists {
		lru.dll.RemoveNode(node)
	} else {
		node := NewLinkedListNode(key)
		lru.mapper[key] = node
	}
	lru.dll.AddTail(lru.mapper[key])
}

// EvictKey method for LRUEvictionPolicyWithCustomDataStructure
func (lru *LRUEvictionPolicyWithCustomDataStructure) EvictKey() interface{} {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if len(lru.mapper) == 0 {
		return nil
	}

	node := lru.dll.GetNodeAtHead()
	delete(lru.mapper, node.element)
	return node.element
}

// LinkedListNode struct
type LinkedListNode struct {
	element interface{}
	prev    *LinkedListNode
	next    *LinkedListNode
}

// NewLinkedListNode creates a new LinkedListNode instance
func NewLinkedListNode(element interface{}) *LinkedListNode {
	return &LinkedListNode{element: element}
}

// DoubleLinkedList struct
type DoubleLinkedList struct {
	head *LinkedListNode
	tail *LinkedListNode
	mu   sync.Mutex
}

// NewDoubleLinkedList creates a new DoubleLinkedList instance
func NewDoubleLinkedList() *DoubleLinkedList {
	head := NewLinkedListNode(nil)
	tail := NewLinkedListNode(nil)
	head.next = tail
	tail.prev = head

	return &DoubleLinkedList{
		head: head,
		tail: tail,
	}
}

// RemoveNode method for DoubleLinkedList
func (dll *DoubleLinkedList) RemoveNode(node *LinkedListNode) {
	dll.mu.Lock()
	defer dll.mu.Unlock()

	node.prev.next = node.next
	node.next.prev = node.prev
	node.next = nil
	node.prev = nil
}

// GetNodeAtHead method for DoubleLinkedList
func (dll *DoubleLinkedList) GetNodeAtHead() *LinkedListNode {
	dll.mu.Lock()
	defer dll.mu.Unlock()

	if dll.head.next == dll.tail {
		return nil
	}
	return dll.head.next
}

// AddTail method for DoubleLinkedList
func (dll *DoubleLinkedList) AddTail(node *LinkedListNode) {
	dll.mu.Lock()
	defer dll.mu.Unlock()

	node.prev = dll.tail.prev
	node.next = dll.tail
	dll.tail.prev.next = node
	dll.tail.prev = node
}

// StorageFullException struct
type StorageFullException struct {
	message string
}

func (e *StorageFullException) Error() string {
	return e.message
}

// DataNotFoundException struct
type DataNotFoundException struct {
	message string
}

func (e *DataNotFoundException) Error() string {
	return e.message
}

// Cache struct
type Cache struct {
	storage       Storage
	evictionPolicy EvictionPolicy
	mu            sync.Mutex
}

// NewCache creates a new Cache instance
func NewCache(storage Storage, evictionPolicy EvictionPolicy) *Cache {
	return &Cache{
		storage:       storage,
		evictionPolicy: evictionPolicy,
	}
}

// Put method for Cache
func (c *Cache) Put(key interface{}, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			if evictedKey := c.evictionPolicy.EvictKey(); evictedKey != nil {
				c.storage.Remove(evictedKey)
				c.Put(key, value)
			}
		}
	}()

	c.storage.Put(key, value)
	c.evictionPolicy.KeyAccessed(key)
}

// Get method for Cache
func (c *Cache) Get(key interface{}) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock

()

	defer func() {
		if r := recover(); r != nil {
			// handle panic if needed
		}
	}()

	value, err := c.storage.Get(key)
	if err == nil {
		c.evictionPolicy.KeyAccessed(key)
		return value
	}

	return nil
}

// CacheProvider struct
type CacheProvider struct{}

// DefaultCache method for CacheProvider
func (cp *CacheProvider) DefaultCache(capacity int) *Cache {
	return NewCache(NewMapStorage(capacity), NewLRUEvictionPolicyWithCustomDataStructure())
}

// PutResponse struct
type PutResponse struct {
	TotalTime float64
}

// GetResponse struct
type GetResponse struct {
	TotalTime float64
	Value     interface{}
}

// LevelCache interface
type LevelCache interface {
	Put(key interface{}, value interface{}) PutResponse
	Get(key interface{}) GetResponse
}

// CacheMetadata struct
type CacheMetadata struct {
	ReadTime  float64
	WriteTime float64
}

// DefaultCache struct
type DefaultCache struct {
	cache    *Cache
	metadata CacheMetadata
	next     LevelCache
	mu       sync.Mutex
}

// NewDefaultCache creates a new DefaultCache instance
func NewDefaultCache(cache *Cache, metadata CacheMetadata, next LevelCache) *DefaultCache {
	return &DefaultCache{
		cache:    cache,
		metadata: metadata,
		next:     next,
	}
}

// Put method for DefaultCache
func (dc *DefaultCache) Put(key interface{}, value interface{}) PutResponse {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	totalTime := 0.0
	oldValue := dc.cache.Get(key)
	totalTime += dc.metadata.ReadTime

	if oldValue != value {
		dc.cache.Put(key, value)
		totalTime += dc.metadata.WriteTime
		totalTime += dc.next.Put(key, value).TotalTime
	}

	return PutResponse{TotalTime: totalTime}
}

// Get method for DefaultCache
func (dc *DefaultCache) Get(key interface{}) GetResponse {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	totalTime := 0.0
	value := dc.cache.Get(key)
	totalTime += dc.metadata.ReadTime

	if value == nil {
		nextResponse := dc.next.Get(key)
		totalTime += nextResponse.TotalTime
		value = nextResponse.Value

		if value != nil {
			dc.cache.Put(key, value)
			totalTime += dc.metadata.WriteTime
		}
	}

	return GetResponse{TotalTime: totalTime, Value: value}
}

// NullCache struct
type NullCache struct{}

// Put method for NullCache
func (nc *NullCache) Put(key interface{}, value interface{}) PutResponse {
	return PutResponse{TotalTime: 0.0}
}

// Get method for NullCache
func (nc *NullCache) Get(key interface{}) GetResponse {
	return GetResponse{TotalTime: 0.0, Value: nil}
}

// StatsResponse struct
type StatsResponse struct {
	AvgReadTime  float64
	AvgWriteTime float64
}

// MultilevelCacheService struct
type MultilevelCacheService struct {
	l1Cache    LevelCache
	readTimes  []float64
	writeTimes []float64
	size       int
	mu         sync.Mutex
}

// NewMultilevelCacheService creates a new MultilevelCacheService instance
func NewMultilevelCacheService(l1Cache LevelCache, size int) *MultilevelCacheService {
	return &MultilevelCacheService{
		l1Cache:    l1Cache,
		size:       size,
		readTimes:  make([]float64, 0),
		writeTimes: make([]float64, 0),
	}
}

// Get method for MultilevelCacheService
func (mcs *MultilevelCacheService) Get(key interface{}) GetResponse {
	mcs.mu.Lock()
	defer mcs.mu.Unlock()

	getResponse := mcs.l1Cache.Get(key)
	mcs.addToReads(getResponse.TotalTime)
	return getResponse
}

// Put method for MultilevelCacheService
func (mcs *MultilevelCacheService) Put(key interface{}, value interface{}) PutResponse {
	mcs.mu.Lock()
	defer mcs.mu.Unlock()

	putResponse := mcs.l1Cache.Put(key, value)
	mcs.addToWrite(putResponse.TotalTime)
	return putResponse
}

// GetReadAvg method for MultilevelCacheService
func (mcs *MultilevelCacheService) GetReadAvg() float64 {
	mcs.mu.Lock()
	defer mcs.mu.Unlock()

	return mcs.calculateAvg(mcs.readTimes)
}

// GetWriteAvg method for MultilevelCacheService
func (mcs *MultilevelCacheService) GetWriteAvg() float64 {
	mcs.mu.Lock()
	defer mcs.mu.Unlock()

	return mcs.calculateAvg(mcs.writeTimes)
}

// GetStats method for MultilevelCacheService
func (mcs *MultilevelCacheService) GetStats() StatsResponse {
	mcs.mu.Lock()
	defer mcs.mu.Unlock()

	return StatsResponse{
		AvgReadTime:  mcs.calculateAvg(mcs.readTimes),
		AvgWriteTime: mcs.calculateAvg(mcs.writeTimes),
	}
}

func (mcs *MultilevelCacheService) addToReads(totalTime float64) {
	mcs.readTimes = append(mcs.readTimes, totalTime)
	for len(mcs.readTimes) > mcs.size {
		mcs.readTimes = mcs.readTimes[1:]
	}
}

func (mcs *MultilevelCacheService) addToWrite(totalTime float64) {
	mcs.writeTimes = append(mcs.writeTimes, totalTime)
	for len(mcs.writeTimes) > mcs.size {
		mcs.writeTimes = mcs.writeTimes[1:]
	}
}

func (mcs *MultilevelCacheService) calculateAvg(times []float64) float64 {
	if len(times) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, t := range times {
		sum += t
	}

	return sum / float64(len(times))
}

func main() {
	// Usage example
	cacheProvider := &CacheProvider{}
	multilevelCache := NewMultilevelCacheService(cacheProvider.DefaultCache(5), 5)

	key := "example_key"
	value := "example_value"

	// Put data into the cache
	putResponse := multilevelCache.Put(key, value)
	fmt.Printf("Put Response: Total Time = %f\n", putResponse.TotalTime)

	// Get data from the cache
	getResponse := multilevelCache.Get(key)
	fmt.Printf("Get Response: Total Time = %f, Value = %v\n", getResponse.TotalTime, getResponse.Value)

	// Get cache statistics
	stats := multilevelCache.GetStats()
	fmt.Printf("Cache Stats: Avg Read Time = %f, Avg Write Time = %f\n", stats.AvgReadTime, stats.AvgWriteTime)
}
