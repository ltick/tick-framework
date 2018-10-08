package block

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ltick/tick-framework/utility/kvstore"
)

var (
	errReadSize error = fmt.Errorf("block: read size error")
)

type ContentWriter interface {
	Write(key []byte, value []byte) (index *Index, err error)
	Close() error
	Size() int64
}

type contentWriter struct {
	*kvstore.KVStore
	file     *os.File
	basepath string
	size     int64
}

func NewContentWriter(filename, basepath string) (w ContentWriter, err error) {
	var (
		file   *os.File
		offset int64
	)
	if file, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0644)); err != nil {
		return
	}
	if offset, err = file.Seek(0, os.SEEK_END); err != nil {
		return
	}
	w = &contentWriter{
		KVStore:  kvstore.NewKVStore(kvstore.NopStore, file),
		file:     file,
		basepath: basepath,
		size:     offset,
	}
	return
}

func (w *contentWriter) Write(key []byte, value []byte) (index *Index, err error) {
	var data *kvstore.KVStoreData = kvstore.NewKVStoreData(key, value)
	if err = w.Set(data); err != nil {
		return
	}
	// index信息定义为 文件名 StoreData的offset，StoreData的length
	index = NewIndex(strings.TrimPrefix(w.file.Name(), w.basepath), uint64(w.size), data.Size())
	w.size += int64(data.Size())
	return
}

func (w *contentWriter) Close() error {
	return w.file.Close()
}

func (w *contentWriter) Size() int64 {
	return w.size
}

type ContentReader interface {
	Read(offset uint64, length uint32) (value []byte, err error)
	SequentialRead() (key string, index *Index, err error)
	Close() error
}

type contentReader struct {
	*kvstore.KVStore
	file     *os.File
	basepath string
	offset   int64
}

func NewContentReader(filename, basepath string) (r ContentReader, err error) {
	var file *os.File
	if file, err = os.OpenFile(filename, os.O_RDONLY, os.FileMode(0644)); err != nil {
		return
	}
	r = &contentReader{
		KVStore:  kvstore.NewKVStore(file, kvstore.NopStore),
		file:     file,
		basepath: basepath,
		offset:   0,
	}
	return
}

func (r *contentReader) Read(offset uint64, length uint32) (value []byte, err error) {
	var data *kvstore.KVStoreData
	if data, err = r.Get(int64(offset)); err != nil {
		return
	}
	if data.Size() != length {
		err = errReadSize
		return
	}
	value = make([]byte, data.ValueLength())
	copy(value, data.Value())
	return
}

func (r *contentReader) SequentialRead() (key string, index *Index, err error) {
	var data *kvstore.KVStoreData
	if data, err = r.Get(r.offset); err != nil {
		return
	}
	key = data.Key()
	index = NewIndex(strings.TrimPrefix(r.file.Name(), r.basepath), uint64(r.offset), data.Size())
	r.offset += int64(data.Size())
	return
}

func (r *contentReader) Close() error {
	return r.file.Close()
}

type IndexWriter interface {
	Write(key string, index *Index) (err error)
	Close() error
	WriteAndBuffered()
	BufferWriteTo(w *indexWriter) (n int64, err error)
}

type indexWriter struct {
	*kvstore.KVStore
	file *fileBufferWriteSeeker
}

type fileBufferWriteSeeker struct {
	file   *os.File
	buffer *bytes.Buffer
	writer io.Writer
}

func newFileBufferWriteSeeker(file *os.File) *fileBufferWriteSeeker {
	return &fileBufferWriteSeeker{
		file:   file,
		buffer: bytes.NewBuffer(nil),
		writer: file,
	}
}

func (w *fileBufferWriteSeeker) Write(p []byte) (n int, err error) {
	return w.writer.Write(p)
}

func (w *fileBufferWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	return w.file.Seek(offset, whence)
}

func (w *fileBufferWriteSeeker) Close() error {
	return w.file.Close()
}

func (w *fileBufferWriteSeeker) WriteAndBuffered() {
	w.writer = io.MultiWriter(w.file, w.buffer)
}

func (w *fileBufferWriteSeeker) GetBuffered() *bytes.Buffer {
	return w.buffer
}

func NewIndexWriter(filename string, trunc bool) (w *indexWriter, err error) {
	var (
		file *os.File
		flag int = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	)
	if trunc {
		flag |= os.O_TRUNC
	}
	if file, err = os.OpenFile(filename, flag, os.FileMode(0644)); err != nil {
		return
	}
	var fileBuffer *fileBufferWriteSeeker = newFileBufferWriteSeeker(file)
	w = &indexWriter{
		KVStore: kvstore.NewKVStore(kvstore.NopStore, fileBuffer),
		file:    fileBuffer,
	}
	return
}

func (w *indexWriter) Write(key string, index *Index) (err error) {
	var (
		value []byte
		data  *kvstore.KVStoreData
	)
	if value, err = index.MarshalBinary(); err != nil {
		return
	}
	data = kvstore.NewKVStoreData([]byte(key), value)
	if err = w.Set(data); err != nil {
		return
	}
	return
}

func (w *indexWriter) Close() error {
	return w.file.Close()
}

func (w *indexWriter) WriteAndBuffered() {
	w.file.WriteAndBuffered()
}

func (b *indexWriter) BufferWriteTo(w *indexWriter) (n int64, err error) {
	return b.file.GetBuffered().WriteTo(w.file)
}

type IndexReader interface {
	SequentialRead() (key string, index *Index, err error)
	Close() error
}

type indexReader struct {
	*kvstore.KVStore
	file   *os.File
	offset int64
}

func NewIndexReader(filename string) (r IndexReader, err error) {
	var file *os.File
	if file, err = os.OpenFile(filename, os.O_RDONLY, os.FileMode(0644)); err != nil {
		return
	}
	r = &indexReader{
		KVStore: kvstore.NewKVStore(file, kvstore.NopStore),
		file:    file,
		offset:  0,
	}
	return
}

func (r *indexReader) SequentialRead() (key string, index *Index, err error) {
	var data *kvstore.KVStoreData
	if data, err = r.Get(r.offset); err != nil {
		return
	}
	key = data.Key()
	index = &Index{}
	if index.UnmarshalBinary(data.Value()); err != nil {
		return
	}
	r.offset += int64(data.Size())
	return
}

func (r *indexReader) Close() error {
	return r.file.Close()
}

func LoadIndexFromFile(filename string) (indexTable map[string]*Index, err error) {
	indexTable = make(map[string]*Index)
	var indexReader IndexReader
	if indexReader, err = NewIndexReader(filename); err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}
	defer indexReader.Close()
	for {
		var (
			key   string
			index *Index
		)
		if key, index, err = indexReader.SequentialRead(); err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		if index.IsDel() {
			delete(indexTable, key)
		} else {
			indexTable[key] = index
		}
	}
	return
}

func RangeIndexFromFile(filename string, doFunc func(key string, index *Index)) (err error) {
	var indexReader IndexReader
	if indexReader, err = NewIndexReader(filename); err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}
	defer indexReader.Close()
	for {
		var (
			key   string
			index *Index
		)
		if key, index, err = indexReader.SequentialRead(); err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		doFunc(key, index)
	}
	return
}
