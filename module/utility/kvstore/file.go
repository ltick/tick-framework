package kvstore

import (
	"errors"
	"os"
)

type KVFileStore struct {
	*KVStore
	file *os.File
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
	}
	return s, nil
}

func (f *KVFileStore) Write(v *KVStoreData) (err error) {
	var data []byte
	if err = f.KVStore.Set(v); err != nil {
		return errors.New(err.Error())
	}
	if data, err = v.MarshalBinary(); err != nil {
		return
	}
	if _, err = f.file.Write(data); err != nil {
		return errors.New(err.Error())
	}
	return nil
}

func (f *KVFileStore) Read(offset uint32) (v *KVStoreData, err error) {
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
	return
}

func (f *KVFileStore) Close() error {
	return f.file.Close()
}
