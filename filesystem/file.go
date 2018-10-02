package filesystem

import (
	"context"
	"fmt"

	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/filesystem/block"
)

var (
	errFileNotExist error = fmt.Errorf("filesystem file: lru key not exist")
)

type FileHandler struct {
	block *block.Instance
}

func NewFileHandler() Handler {
	return &FileHandler{
		block: block.NewInstance(),
	}
}

func (this *FileHandler) Initiate(ctx context.Context, conf *config.Instance) (err error) {
	if ctx, err = this.block.Initiate(ctx); err != nil {
		return
	}
	if ctx, err = this.block.OnStartup(ctx); err != nil {
		return
	}
	return
}

func (this *FileHandler) SetContent(key string, content []byte) (err error) {
	if _, err = this.block.Write([]byte(key), content); err != nil {
		return
	}
	return
}

func (this *FileHandler) GetContent(key string) (content []byte, err error) {
	var (
		index      *block.Index = new(block.Index)
	)
	return this.block.Read(index)
}