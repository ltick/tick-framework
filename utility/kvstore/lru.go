package kvstore

import (
	"container/list"
	"context"
	//"fmt"
	"bufio"
	"encoding"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ltick/tick-framework/module/config"
)

type KVLruFileStore struct {
	sync.RWMutex

	list     *list.List
	table    map[string]*list.Element
	size     uint64 // 当前大小
	capacity uint64 // 总容量
	tempDir  string
	saveItem chan KVStore
}

func NewKVLruFileStore() *KVLruFileStore {
	return &KVLruFileStore{
		list:     list.New(),
		table:    make(map[string]*list.Element),
		size:     0,
		saveItem: make(chan KVStore),
	}
}

func (lru *KVLruFileStore) Initiate(ctx context.Context, conf *config.Instance) (err error) {
	var (
		tempDir      string        = conf.GetString("LRU_DIR")
		capacity     uint64        = uint64(conf.GetInt64("LRU_CAPACITY"))
		saveInterval time.Duration = conf.GetDuration("LRU_SAVE_INTERVAL")
	)
	if err = os.MkdirAll(tempDir, os.FileMode(0755)); err != nil {
		return
	}
	lru.capacity = capacity
	lru.tempDir = tempDir
	// 初始化
	if err = lru.load(); err != nil {
		return
	}
	//fmt.Println(lru.size, lru.list.Len())
	// 类似redis的持久化存储
	// AOF数据格式和RDB相同, 所以直接追加到文件
	go func() {
		var (
			encoder EncodeCloser
			err     error
		)
		if encoder, err = NewWriteFile(filepath.Join(lru.tempDir, "rdb")); err != nil {
			return
		}
		for {
			var item KVStore
			select {
			case item = <-lru.saveItem: // 追加
				if err = encoder.Encode(item); err != nil {
					continue
				}
			case <-time.Tick(saveInterval): // 定期保存
				// TODO 写tmp文件, 再切换句柄
				func() {
					lru.Lock()
					defer lru.Unlock()

					if err = encoder.Truncate(); err != nil {
						return
					}
					var element *list.Element
					for element = lru.list.Back(); element != nil; element = element.Prev() { // 从后往前写
						if err = encoder.Encode(element.Value.(KVStore)); err != nil {
							continue
						}
					}
					return
				}()
			}
		}
	}()
	return
}

func (lru *KVLruFileStore) load() (err error) {
	lru.Lock()
	defer lru.Unlock()

	var (
		fr    *KVStoreReader
		value *KVStore = new(KVStore)
	)
	if fr, err = NewKVStoreReader(filepath.Join(lru.tempDir, "rdb")); err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}
	defer fr.Close()
	for {
		if err = fr.ReadOne(value); err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		lru.set(value.Key(), value.Value())
	}
	return
}

func (lru *KVLruFileStore) Peek(key string) (v []byte, ok bool) {
	lru.RLock()
	defer lru.RUnlock()

	var element *list.Element
	if element, ok = lru.table[key]; !ok {
		return
	}
	v = element.Value.(KVStore).Value()
	return
}

func (lru *KVLruFileStore) Update(key string, value []byte) {
	lru.Lock()
	defer lru.Unlock()

	var (
		item    KVStore = NewKVStore([]byte(key), value)
		element *list.Element
		ok      bool
	)
	if element, ok = lru.table[key]; !ok {
		return
	}
	lru.updateInplace(element, item)
	lru.saveItem <- item
}

func (lru *KVLruFileStore) Get(key string) (v []byte, ok bool) {
	lru.RLock()
	defer lru.RUnlock()

	var element *list.Element
	if element, ok = lru.table[key]; !ok {
		return
	}
	lru.moveToFront(element)
	v = element.Value.(KVStore).Value()
	return
}

func (lru *KVLruFileStore) Set(key string, value []byte) {
	lru.Lock()
	defer lru.Unlock()

	lru.set(key, value)
	// 追加
	lru.saveItem <- NewKVStore([]byte(key), value)
}

func (lru *KVLruFileStore) set(key string, value []byte) {
	var (
		item    KVStore = NewKVStore([]byte(key), value)
		element *list.Element
		ok      bool
	)
	if element, ok = lru.table[key]; ok {
		lru.updateInplace(element, item)
	} else {
		lru.addNew(key, item)
	}
}

func (lru *KVLruFileStore) Delete(key string) (ok bool) {
	// TODO 删除时, AOF没有删掉
	lru.Lock()
	defer lru.Unlock()

	var element *list.Element
	if element, ok = lru.table[key]; !ok {
		return
	}
	lru.list.Remove(element)
	delete(lru.table, key)
	lru.size -= uint64(element.Value.(KVStore).Size())
	return
}

func (lru *KVLruFileStore) updateInplace(element *list.Element, other KVStore) {
	var item KVStore = element.Value.(KVStore)
	element.Value = other
	lru.size -= uint64(item.Size())
	lru.size += uint64(other.Size())
	lru.moveToFront(element)
	lru.checkCapacity()
}

func (lru *KVLruFileStore) moveToFront(element *list.Element) {
	lru.list.MoveToFront(element)
}

func (lru *KVLruFileStore) addNew(key string, item KVStore) {
	var element *list.Element = lru.list.PushFront(item)
	lru.table[item.Key()] = element
	lru.size += uint64(item.Size())
	lru.checkCapacity()
}

func (lru *KVLruFileStore) checkCapacity() {
	var (
		delElem  *list.Element
		delValue interface{}
	)
	for lru.size > lru.capacity {
		if delElem = lru.list.Back(); delElem == nil {
			return
		}
		delValue = lru.list.Remove(delElem)
		delete(lru.table, delValue.(KVStore).Key())
		lru.size -= uint64(delValue.(KVStore).Size())
	}
}

