package kvstore

import (
	"bufio"
	"errors"
	"os"
	"encoding/binary"
)

type KVFileStore struct {
	*KVStore
	file *os.File
	br   *bufio.Reader
	bw   *bufio.Writer
	offset int
}

func NewKVFileStore(name string) (s *KVFileStore, err error) {
	var (
		file *os.File
	)
	fileReader, err := os.OpenFile(name, os.O_RDONLY, os.FileMode(0644))
	if err != nil {
		return nil, errors.New(err.Error())
	}
	fileWriter, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.FileMode(0644))
	if err != nil {
		return nil, errors.New(err.Error())
	}
	kvStore, err := NewKVStore(fileReader, fileWriter)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	s = &KVFileStore{
		KVStore: kvStore,
		file:    file,
		br:      bufio.NewReader(fileReader),
		bw:      bufio.NewWriter(fileWriter),
	}
	return s, nil
}
func (fr *KVFileStore) ReadOne() (item *KVStoreData, err error) {
	var data []byte
	data, err = fr.readOne()
	if  err != nil {
		return
	}
	item, err = NewKVStoreDataFromBinary(data)
	if  err != nil {
		return
	}
	return item, nil
}
func (fr *KVFileStore) readOne() (data []byte, err error) {
	var (
		buf         []byte
		keyLength   uint32
		valueLength uint32
	)
	if buf, err = fr.br.Peek(10); err != nil {
		return
	}
	keyLength = binary.BigEndian.Uint32(buf[2:6])
	valueLength = binary.BigEndian.Uint32(buf[6:10])
	data = make([]byte, 10+keyLength+valueLength)
	var (
		bufferBytes []byte
		readN       int
	)
	for bufferBytes = data; len(bufferBytes) > 0; bufferBytes = bufferBytes[readN:] {
		if readN, err = fr.br.Read(bufferBytes); err != nil {
			return
		}
		fr.offset += readN
	}
	return
}
func (fr *KVFileStore) Offset() int {
	return fr.offset
}
func (f *KVFileStore) Write(v *KVStoreData) (err error) {
	var data []byte
	if err = f.KVStore.Set(v); err != nil {
		return errors.New(err.Error())
	}
	if data, err = v.MarshalBinary(); err != nil {
		return
	}
	if _, err = f.bw.Write(data); err != nil {
		return
	}
	if err = f.bw.Flush(); err != nil {
		return
	}
	return nil
}

func (f *KVFileStore) Read(offset int64) (v *KVStoreData, err error) {
	v, err = f.KVStore.Get(offset)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	return v, nil
}

func (f *KVFileStore) Truncate() (err error) {
	if err = f.file.Truncate(0); err != nil {
		return
	}
	if _, err = f.file.Seek(0, os.SEEK_SET); err != nil {
		return
	}
	f.bw.Reset(f.file)
	return
}

func (f *KVFileStore) Close() error {
	return f.file.Close()
}
