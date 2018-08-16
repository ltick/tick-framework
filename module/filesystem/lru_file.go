package filesystem

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/filesystem/block"
)

var (
	errLRUFileNotExist error = fmt.Errorf("filesystem lru file: key not exist")
)

type LRUFileHandler struct {
	blockInstance *block.Instance
	lru           *lruInstance // 对key做lru, 限制内存中key的数量
}

func NewLRUFileHandler() Handler {
	return &LRUFileHandler{
		blockInstance: block.NewInstance(),
	}
}

func (this *LRUFileHandler) Initiate(ctx context.Context, conf *config.Instance) (err error) {
	var (
		lruCapacity           int64         = conf.GetInt64("FILESYSTEM_LRU_CAPACITY")
		defragContentInterval time.Duration = conf.GetDuration("FILESYSTEM_DEFRAG_CONTENT_INTERVAL")
		defragContentLifetime time.Duration = conf.GetDuration("FILESYSTEM_DEFRAG_CONTENT_LIFETIME")
	)
	if ctx, err = this.blockInstance.Initiate(ctx); err != nil {
		return
	}
	this.lru = newLRUInstance(uint64(lruCapacity), func(key string) {})
	// 初始化LRU

	if ctx, err = this.blockInstance.OnStartup(ctx); err != nil {
		return
	}
	go func() {
		for range time.Tick(defragContentInterval) {
			this.blockInstance.DefragContent(defragContentLifetime)
		}
	}()
	return
}

func (this *LRUFileHandler) SetContent(key string, content []byte) (err error) {
	if err = this.blockInstance.Set(key, content); err != nil {
		return
	}
	this.lru.Add(key)
	return
}

func (this *LRUFileHandler) GetContent(key string) (content []byte, err error) {
	if content, err = this.blockInstance.Get(key); err != nil {
		return
	}
	this.lru.Add(key)
	return
}

type lruInstance struct {
	sync.Mutex
	list     *list.List
	table    map[string]*list.Element
	size     uint64           // 当前key数量
	capacity uint64           // 总容量
	delFunc  func(key string) // lru删除时, 调用的函数
}

func newLRUInstance(capacity uint64, delFunc func(key string)) *lruInstance {
	return &lruInstance{
		list:     list.New(),
		table:    make(map[string]*list.Element),
		size:     0,
		capacity: capacity,
		delFunc:  delFunc,
	}
}

func (lru *lruInstance) Add(key string) {
	lru.Lock()
	defer lru.Unlock()

	var (
		element *list.Element
		ok      bool
	)
	if element, ok = lru.table[key]; ok {
		lru.list.MoveToFront(element)
	} else {
		element = lru.list.PushFront(key)
		lru.table[key] = element
		lru.size++
	}
	lru.checkCapacity()
}

func (lru *lruInstance) checkCapacity() {
	var (
		delElement *list.Element
		delValue   interface{}
	)
	for lru.size > lru.capacity {
		if delElement = lru.list.Back(); delElement == nil {
			return
		}
		delValue = lru.list.Remove(delElement)
		delete(lru.table, delValue.(string))
		lru.delFunc(delValue.(string))
		lru.size--
	}
}
