package block

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/ltick/tick-framework/utility/kvstore"
	"github.com/ltick/tick-framework/utility/pooling"
	"github.com/ltick/tick-framework/module/config"
)

var (
	errMaxSize error = fmt.Errorf("block: file max size")
)

const (
	FilenameSuffixInSecond string = "20060102150405"
)

type Block struct {
	tempDir string
	maxSize int64

	writelogerPool pooling.Pool
}

func NewFileBlockHandler() BlockHandler {
	return &Block{}
}

func (b *Block) Initiate(ctx context.Context, conf *config.Instance) (err error) {
	var (
		tempDir string = conf.GetString("FILESYSTEM_BLOCK_DIR")
		maxSize int64  = conf.GetInt64("FILESYSTEM_BLOCK_SIZE")
		maxIdle int    = conf.GetInt("FILESYSTEM_BLOCK_IDLE")
	)
	if err = os.MkdirAll(tempDir, os.FileMode(0755)); err != nil {
		return
	}
	b.tempDir = tempDir
	b.maxSize = maxSize

	b.writelogerPool = pooling.NewPool(
		maxIdle,
		func() (conn pooling.Conn, err error) {
			var (
				// 生成文件名
				filename   string = filepath.Join(tempDir, fmt.Sprintf("%s.%d", time.Now().Format(FilenameSuffixInSecond), rand.Uint64()))
				fileWriter WriteCloser
			)
			if fileWriter, err = OpenFileWriteCloser(filename, true); err != nil {
				return
			}
			conn = pooling.NewConn(fileWriter, fileWriter.Close)
			return
		},
		func(c pooling.Conn, t time.Time) error {
			return c.Do(func(idle interface{}) error {
				// 判断文件大小、超过限制，返回错误
				if maxSize > 0 && idle.(WriteCloser).Offset() >= maxSize {
					return errMaxSize
				}
				return nil
			})
		},
	)

	// 初始化 将上次退出时的文件句柄放入writelogerPool中
	err = filepath.Walk(tempDir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		var fileWriter WriteCloser
		if fileWriter, err = OpenFileWriteCloser(filepath.Join(tempDir, f.Name()), false); err != nil {
			return err
		}
		if maxSize > 0 && fileWriter.Offset() >= maxSize {
			return nil
		}
		b.writelogerPool.Put(pooling.NewConn(fileWriter, fileWriter.Close).BindPool(b.writelogerPool))
		return nil
	})
	return
}

func (v *Block) Write(key, value []byte) (index *Index, err error) {
	var writelogerConn pooling.Conn
	if writelogerConn, err = v.writelogerPool.Get(); err != nil {
		return
	}
	writelogerConn.Do(func(idle interface{}) (err error) {
		if index, err = idle.(WriteCloser).Write(key, value); err != nil {
			return
		}
		return
	})
	writelogerConn.Recycle()
	return
}

// TODO 缓存句柄
func (v *Block) Read(index *Index) (data []byte, err error) {
	var fr ReadCloser
	if fr, err = OpenFileReadCloser(index.Filename()); err != nil {
		return
	}
	defer fr.Close()
	return fr.Read(index.offset, index.length)
}

// vlog整理
// set相同key的时候, 之前key的index会更新, 但是vlog还保留着value
// lru过期时, vlog还保留着value
func (v *Block) Defrag(defragDuration time.Duration, rebuildIndex func(key string, index *Index)) {
	var defragTimestamp time.Time = time.Now().Add(-defragDuration)

	filepath.Walk(v.tempDir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		if f.Size() < v.maxSize {
			return nil
		}
		// N天之内的vlog不整理
		if f.ModTime().After(defragTimestamp) {
			return nil
		}
		var (
			filename string = filepath.Join(v.tempDir, f.Name())
			kvReader *kvstore.KVStoreReader
			item     *kvstore.KVStore = new(kvstore.KVStore)
		)
		if kvReader, err = kvstore.NewKVStoreReader(filename); err != nil {
			return err
		}
		for { // 遍历Key, 判断是否重建索引
			if err = kvReader.ReadOne(item); err != nil {
				continue
			}
			// key还在使用, 重新写入vlog, 更新索引
			rebuildIndex(item.Key(), NewIndex(
				filename,
				uint64(kvReader.Offset())+uint64(item.ValueOffset()),
				item.ValueLength(),
			))
		}
		kvReader.Close()
		// 删除旧文件
		os.Remove(filename)
		return nil
	})
}
