package filesystem

import (
	"context"
	"time"

	"github.com/ltick/tick-framework/config"
	"github.com/ltick/tick-framework/filesystem/block"
)

type FileHandler struct {
	blockInstance *block.Instance
}

func NewFileHandler() Handler {
	return &FileHandler{
		blockInstance: block.NewInstance(),
	}
}

func (this *FileHandler) Initiate(ctx context.Context, conf *config.Config) (err error) {
	var (
		defragContentInterval time.Duration = conf.GetDuration("FILESYSTEM_DEFRAG_CONTENT_INTERVAL")
		defragContentLifetime time.Duration = conf.GetDuration("FILESYSTEM_DEFRAG_CONTENT_LIFETIME")
	)
	if ctx, err = this.blockInstance.Initiate(ctx); err != nil {
		return
	}
	if ctx, err = this.blockInstance.OnStartup(ctx); err != nil {
		return
	}
	// 整理Content
	go func() {
		for range time.Tick(defragContentInterval) {
			this.blockInstance.DefragContent(defragContentLifetime)
		}
	}()
	return
}

func (this *FileHandler) SetContent(key string, content []byte) (err error) {
	if err = this.blockInstance.Set(key, content); err != nil {
		return
	}
	return
}

func (this *FileHandler) GetContent(key string) (content []byte, err error) {
	if content, err = this.blockInstance.Get(key); err != nil {
		return
	}
	return
}

func (this *FileHandler) DelContent(key string) (err error) {
	if err = this.blockInstance.Del(key); err != nil {
		return
	}
	return
}
