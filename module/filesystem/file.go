package filesystem

import (
	"context"

	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/filesystem/block"
)

type FileHandler struct {
	blockInstance *block.Instance
}

func NewFileHandler() Handler {
	return &FileHandler{
		blockInstance: block.NewInstance(),
	}
}

func (this *FileHandler) Initiate(ctx context.Context, conf *config.Instance) (err error) {
	if ctx, err = this.blockInstance.Initiate(ctx); err != nil {
		return
	}
	if ctx, err = this.blockInstance.OnStartup(ctx); err != nil {
		return
	}
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
