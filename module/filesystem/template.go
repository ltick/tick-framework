package filesystem

import (
	"context"
	"fmt"
	"time"

	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/filesystem/block"
	"github.com/ltick/tick-framework/module/lru"
)

var (
	errNotExist error = fmt.Errorf("fileTemplate: lru key not exist")
)

type FileTemplateHandler struct {
	blockInstance *block.Instance
	lruInstance   *lru.Instance
}

func NewFileTemplateHandler() TemplateHandler {
	return &FileTemplateHandler{
		blockInstance: block.NewInstance(),
		lruInstance:   lru.NewInstance(),
	}
}

func (this *FileTemplateHandler) Initiate(ctx context.Context, conf *config.Instance) (err error) {
	var (
		defragInterval time.Duration = conf.GetDuration("FILESYSTEM_TEMPLATE_DEFRAG_INTERVAL")
		defragLifetime time.Duration = conf.GetDuration("FILESYSTEM_TEMPLATE_DEFRAG_LIFETIME")
	)
	if ctx, err = this.blockInstance.Initiate(ctx); err != nil {
		return
	}
	if ctx, err = this.blockInstance.OnStartup(ctx); err != nil {
		return
	}
	if ctx, err = this.lruInstance.Initiate(ctx); err != nil {
		return
	}
	if ctx, err = this.lruInstance.OnStartup(ctx); err != nil {
		return
	}
	go func() {
		for range time.Tick(defragInterval) {
			this.blockInstance.Defrag(defragLifetime, func(key string, index *block.Index) {
				this.lruInstance.Lock()
				defer this.lruInstance.Unlock()

				var (
					lruValue []byte
					lruIndex *block.Index = new(block.Index)
					ok       bool
					data     []byte
					err      error
				)
				if lruValue, ok = this.lruInstance.Peek(key); !ok {
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
				this.lruInstance.Update(key, lruValue)
			})
		}
	}()
	return
}

func (this *FileTemplateHandler) SetContent(key string, content []byte) (err error) {
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
	this.lruInstance.Set(key, indexValue)
	return
}

func (this *FileTemplateHandler) GetContent(key string) (content []byte, err error) {
	var (
		indexValue []byte
		ok         bool
		index      *block.Index = new(block.Index)
	)
	if indexValue, ok = this.lruInstance.Get(key); !ok {
		err = errNotExist
		return
	}
	if err = index.UnmarshalBinary(indexValue); err != nil {
		err = errNotExist
		return
	}
	return this.blockInstance.Read(index)
}
