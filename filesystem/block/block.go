package block

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ltick/tick-framework/config"
)

var (
	errNotExist error = fmt.Errorf("block: key not exist")
)

const (
	FilenameSuffixInSecond string = "20060102150405"
	FilenameBlockIndex     string = "blockIndex"
	FilenameBlockIndexTemp string = "blockIndexTemp"
)

type Block struct {
	tempDir       string
	maxSize       int64
	contentMutex  *sync.Mutex
	contentWriter ContentWriter     // 只有一个写句柄
	indexTable    map[string]*Index // 内存index信息
	indexMutex    *sync.RWMutex
	indexWriter   IndexWriter
}

func NewFileBlockHandler() BlockHandler {
	return &Block{
		contentMutex: new(sync.Mutex),
		indexTable:   make(map[string]*Index),
		indexMutex:   new(sync.RWMutex),
	}
}

func (b *Block) Initiate(ctx context.Context, conf *config.Config) (err error) {
	var (
		tempDir      string        = conf.GetString("FILESYSTEM_BLOCK_DIR")
		maxSize      int64         = conf.GetInt64("FILESYSTEM_BLOCK_CONTENT_SIZE")
		dumpInterval time.Duration = conf.GetDuration("FILESYSTEM_BLOCK_INDEX_SAVE_INTERVAL")
	)
	if err = os.MkdirAll(tempDir, os.FileMode(0755)); err != nil {
		return
	}
	b.tempDir = tempDir
	b.maxSize = maxSize

	// 初始化 将上次退出时的文件句柄传入
	if err = filepath.Walk(b.tempDir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		if f.Name() == FilenameBlockIndex || f.Name() == FilenameBlockIndexTemp {
			return nil
		}
		var blockWriter ContentWriter
		if blockWriter, err = NewContentWriter(filepath.Join(b.tempDir, f.Name()), b.tempDir); err != nil {
			return err
		}
		if b.maxSize > 0 && blockWriter.Size() >= b.maxSize {
			blockWriter.Close()
			return nil
		}
		b.contentMutex.Lock()
		if b.contentWriter != nil {
			b.contentWriter.Close()
		}
		b.contentWriter = blockWriter
		b.contentMutex.Unlock()
		return nil
	}); err != nil {
		return
	}
	// 初始化 index句柄
	if b.indexWriter, err = NewIndexWriter(filepath.Join(b.tempDir, FilenameBlockIndex), false); err != nil {
		return
	}
	// 初始化 内存index
	b.loadIndex()
	go func() {
		for range time.Tick(dumpInterval) {
			b.dumpIndex()
		}
	}()
	return
}

func (b *Block) loadIndex() (err error) {
	var indexTable map[string]*Index
	if indexTable, err = LoadIndexFromFile(filepath.Join(b.tempDir, FilenameBlockIndex)); err != nil {
		return
	}
	b.indexMutex.Lock()
	defer b.indexMutex.Unlock()
	var (
		key   string
		index *Index
	)
	for key, index = range indexTable {
		b.indexTable[key] = index
	}
	return
}

func (b *Block) dumpIndex() (err error) {
	// 打开Buffer
	b.indexMutex.Lock()
	b.indexWriter.WriteAndBuffered()
	b.indexMutex.Unlock()

	// 打开原来的index文件, 然后整理合并完key数据后, 再重命名, 切换句柄
	// 不影响内存中原有的map
	var indexTable map[string]*Index
	if indexTable, err = LoadIndexFromFile(filepath.Join(b.tempDir, FilenameBlockIndex)); err != nil {
		return
	}
	var (
		key         string
		index       *Index
		indexWriter *indexWriter
	)
	if indexWriter, err = NewIndexWriter(filepath.Join(b.tempDir, FilenameBlockIndexTemp), true); err != nil {
		return
	}
	for key, index = range indexTable {
		if err = indexWriter.Write(key, index); err != nil {
			continue
		}
	}
	// buff写回, 重命名, 再切换句柄
	b.indexMutex.Lock()
	if _, err = b.indexWriter.BufferWriteTo(indexWriter); err != nil {
		b.indexMutex.Unlock()
		return
	}
	b.indexWriter.Close()
	os.Rename(filepath.Join(b.tempDir, FilenameBlockIndexTemp), filepath.Join(b.tempDir, FilenameBlockIndex))
	b.indexWriter = indexWriter
	b.indexMutex.Unlock()
	return
}

func (b *Block) updateContentWriter() (err error) {
	if b.contentWriter == nil {
		goto update
	}
	// 文件大小超过限制，切换文件句柄
	if b.maxSize > 0 && b.contentWriter.Size() >= b.maxSize {
		b.contentWriter.Close()
		goto update
	}
	return
update:
	var filename string = filepath.Join(b.tempDir, fmt.Sprintf("%s.%d", time.Now().Format(FilenameSuffixInSecond), rand.Uint64()))
	if b.contentWriter, err = NewContentWriter(filename, b.tempDir); err != nil {
		return
	}
	return
}

func (b *Block) Set(key string, value []byte) (err error) {
	var index *Index
	if index, err = b.writeContent([]byte(key), value); err != nil {
		return
	}
	if err = b.writeIndex(key, index); err != nil {
		return
	}
	return
}

func (b *Block) writeContent(key, value []byte) (index *Index, err error) {
	b.contentMutex.Lock()
	defer b.contentMutex.Unlock()
	if err = b.updateContentWriter(); err != nil {
		return
	}
	if index, err = b.contentWriter.Write(key, value); err != nil {
		return
	}
	return
}

func (b *Block) writeIndex(key string, index *Index) (err error) {
	b.indexMutex.Lock()
	defer b.indexMutex.Unlock()

	if err = b.indexWriter.Write(key, index); err != nil { // 同步写文件
		return
	}
	b.indexTable[key] = index
	return
}

func (b *Block) Get(key string) (value []byte, err error) {
	var index *Index
	if index, err = b.readIndex(key); err != nil {
		return
	}
	if value, err = b.readContent(index); err != nil {
		return
	}
	return
}

func (b *Block) readIndex(key string) (index *Index, err error) {
	var ok bool
	b.indexMutex.RLock()
	index, ok = b.indexTable[key]
	b.indexMutex.RUnlock()
	if !ok {
		err = errNotExist
		return
	}
	return
}

func (b *Block) readContent(index *Index) (value []byte, err error) {
	var r ContentReader
	if r, err = NewContentReader(filepath.Join(b.tempDir, index.Filename()), b.tempDir); err != nil {
		return
	}
	defer r.Close()
	return r.Read(index.offset, index.length)
}

func (b *Block) Del(key string) (err error) {
	b.indexMutex.Lock()
	defer b.indexMutex.Unlock()

	if err = b.indexWriter.Write(key, NewDelIndex()); err != nil {
		return
	}
	delete(b.indexTable, key)
	return
}

// content整理
// set相同key的时候, 之前key的index会更新, 但是旧content还保留着
// 删除key时, content还保留着
func (b *Block) DefragContent(defragDuration time.Duration) (err error) {
	var defragTimestamp time.Time = time.Now().Add(-defragDuration)
	if err = filepath.Walk(b.tempDir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		if f.Name() == FilenameBlockIndex || f.Name() == FilenameBlockIndexTemp {
			return nil
		}
		if f.Size() < b.maxSize {
			return nil
		}
		if f.ModTime().After(defragTimestamp) { // 一段时间之内的vlog不整理
			return nil
		}

		// 开始整理文件
		var (
			filename      string = filepath.Join(b.tempDir, f.Name())
			contentReader ContentReader
		)
		if contentReader, err = NewContentReader(filename, b.tempDir); err != nil {
			return nil
		}
	loop:
		for {
			var (
				key   string
				index *Index
				err   error
			)
			if key, index, err = contentReader.SequentialRead(); err != nil {
				if err == io.EOF {
					break loop
				} else {
					continue
				}
			}
			// 重建索引
			b.rebuildIndex(key, index)
		}
		contentReader.Close()
		os.Remove(filename)
		return nil
	}); err != nil {
		return
	}
	return
}

func (b *Block) rebuildIndex(key string, index *Index) {
	var (
		latestIndex *Index
		latestValue []byte
		err         error
	)
	if latestIndex, err = b.readIndex(key); err != nil {
		return
	}
	if !latestIndex.Same(index) {
		return
	}
	if latestValue, err = b.readContent(index); err != nil {
		return
	}
	b.Set(key, latestValue)
}

func (b *Block) Range(doFunc func(key string, exist bool)) (err error) {
	return RangeIndexFromFile(filepath.Join(b.tempDir, FilenameBlockIndex), func(key string, index *Index) {
		doFunc(key, !index.IsDel())
	})
}
