package filesystem

import (
	"context"
	"fmt"
	"time"

	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/filesystem/block"
	"github.com/ltick/tick-framework/utility/kvstore"
)

var (
	errLruFileNotExist error = fmt.Errorf("filesystem file: lru key not exist")
)

type LruFileHandler struct {
	blockInstance *block.Instance
	lruFileStore *kvstore.KVLruFileStore
}

func NewLruFileHandler() Handler {
	return &LruFileHandler{
		blockInstance: block.NewInstance(),
		lruFileStore: kvstore.NewKVLruFileStore(),
	}
}

func (this *LruFileHandler) Initiate(ctx context.Context, conf *config.Config) (err error) {
	var (
		defragInterval time.Duration = conf.GetDuration("FILESYSTEM_LRU_DEFRAG_INTERVAL")
		defragLifetime time.Duration = conf.GetDuration("FILESYSTEM_LRU_DEFRAG_LIFETIME")
	)
	if ctx, err = this.blockInstance.Initiate(ctx); err != nil {
		return
	}
	if ctx, err = this.blockInstance.OnStartup(ctx); err != nil {
		return
	}
	go func() {
		for range time.Tick(defragInterval) {
			this.blockInstance.Defrag(defragLifetime, func(key string, index *block.Index) {
				var (
					lruValue []byte
					lruIndex *block.Index = new(block.Index)
					ok       bool
					data     []byte
					err      error
				)
				if lruValue, ok = this.lruFileStore.Peek(key); !ok {
					return
				}
				if err = lruIndex.UnmarshalBinary(lruValue); err != nil {
					return
				}
				if !lruIndex.Same(index) {
					return
				}
				if data, err = this.blockInstance.Read(index); err != nil {
					return
				}
				if lruIndex, err = this.blockInstance.Write([]byte(key), data); err != nil {
					return
				}
				if lruValue, err = lruIndex.MarshalBinary(); err != nil {
					return
				}
				this.lruFileStore.Update(key, lruValue)
			})
		}
	}()
	return
}

func (this *LruFileHandler) SetContent(key string, content []byte) (err error) {
	var (
		index      *block.Index
		indexValue []byte
	)
	if index, err = this.blockInstance.Write([]byte(key), content); err != nil {
		return
	}
	if indexValue, err = index.MarshalBinary(); err != nil {
		return
	}
	this.lruFileStore.Set(key, indexValue)
	return
}

func (this *LruFileHandler) GetContent(key string) (content []byte, err error) {
	var (
		indexValue []byte
		ok         bool
		index      *block.Index = new(block.Index)
	)
	if indexValue, ok = this.lruFileStore.Get(key); !ok {
		err = errLruFileNotExist
		return
	}
	if err = index.UnmarshalBinary(indexValue); err != nil {
		err = errLruFileNotExist
		return
	}
	return this.blockInstance.Read(index)
}