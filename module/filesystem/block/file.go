package block

import (
	"os"
	"sync"

	"github.com/ltick/tick-framework/utility/kvstore"
)

type WriteCloser interface {
	Write(key []byte, value []byte) (index *Index, err error)
	Close() error
	Offset() int64
}

type fileWriteCloser struct {
	sync.RWMutex
	file   *os.File
	offset int64
}

func OpenFileWriteCloser(filename string, create bool) (fw WriteCloser, err error) {
	var (
		file   *os.File
		offset int64
		flag   int = os.O_WRONLY
	)
	if create {
		flag |= os.O_CREATE
	}
	if file, err = os.OpenFile(filename, flag, os.FileMode(0644)); err != nil {
		return
	}
	if offset, err = file.Seek(0, os.SEEK_END); err != nil {
		return
	}
	fw = &fileWriteCloser{
		RWMutex: sync.RWMutex{},
		file:    file,
		offset:  offset,
	}
	return
}

func (fw *fileWriteCloser) Write(key []byte, value []byte) (index *Index, err error) {
	fw.Lock()
	defer fw.Unlock()

	var (
		item kvstore.KVStore = kvstore.NewKVStore(key, value)
		data []byte
		n    int
	)
	if data, err = item.MarshalBinary(); err != nil {
		return
	}
	if n, err = fw.file.WriteAt(data, fw.offset); err != nil {
		return
	}
	index = NewIndex(
		fw.file.Name(),
		uint64(fw.offset)+uint64(item.ValueOffset()),
		item.ValueLength(),
	)
	fw.offset += int64(n)
	return
}

func (fw *fileWriteCloser) Close() error {
	return fw.file.Close()
}

func (fw *fileWriteCloser) Offset() int64 {
	return fw.offset
}

type ReadCloser interface {
	Read(offset uint64, length uint32) (data []byte, err error)
	Close() error
}

type fileReadCloser struct {
	sync.RWMutex
	file *os.File
}

func OpenFileReadCloser(filename string) (fr ReadCloser, err error) {
	var file *os.File
	if file, err = os.OpenFile(filename, os.O_RDONLY, os.FileMode(0644)); err != nil {
		return
	}
	fr = &fileReadCloser{
		RWMutex: sync.RWMutex{},
		file:    file,
	}
	return
}

func (fr *fileReadCloser) Read(offset uint64, length uint32) (data []byte, err error) {
	fr.RLock()
	defer fr.RUnlock()

	data = make([]byte, length)
	if _, err = fr.file.ReadAt(data, int64(offset)); err != nil {
		return
	}
	return
}

func (fr *fileReadCloser) Close() error {
	return fr.file.Close()
}
