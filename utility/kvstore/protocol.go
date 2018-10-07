package kvstore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	errIllegalMagicNumber error = fmt.Errorf("kvstore: illegal magic_num")
	errInvalidLength      error = fmt.Errorf("kvstore: invalid length")
)

const (
	KVSTORE_MAGIC_NUM uint16 = 0xF96E
)

var (
	bufPool sync.Pool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

type KVStore struct {
	reader io.ReadSeeker
	writer io.WriteSeeker
}

func NewKVStore(reader io.ReadSeeker, writer io.WriteSeeker) (fr *KVStore, err error) {
	fr = &KVStore{
		reader: reader,
		writer: writer,
	}
	return
}

func (fr *KVStore) get(offset int64) (data []byte, err error) {
	var (
		overheadLength uint32 = 10
		buf            []byte = make([]byte, overheadLength)
		keyLength      uint32
		valueLength    uint32
	)
	_, err = fr.reader.Seek(offset, os.SEEK_CUR)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	_, err = fr.reader.Read(buf)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	keyLength = binary.BigEndian.Uint32(buf[2:6])
	valueLength = binary.BigEndian.Uint32(buf[6:10])
	data = make([]byte, overheadLength+keyLength+valueLength)
	var (
		bufferBytes []byte
		readN       int
	)
	for bufferBytes = data; len(bufferBytes) > 0; bufferBytes = bufferBytes[readN:] {
		if readN, err = fr.reader.Read(bufferBytes); err != nil {
			return nil, errors.New(err.Error())
		}
	}
	return
}

func (fr *KVStore) Get(offset int64) (s *KVStoreData, err error) {
	var data []byte
	if data, err = fr.get(offset); err != nil {
		return
	}
	s, err = NewKVStoreDataFromBinary(data)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	return s, nil
}

func (fr *KVStore) Set(s *KVStoreData) (err error) {
	var data []byte
	if data, err = s.MarshalBinary(); err != nil {
		return errors.New(err.Error())
	}
	_, err = fr.writer.Seek(0, os.SEEK_END)
	if err != nil {
		return  errors.New(err.Error())
	}
	if _, err = fr.writer.Write(data); err != nil {
		return errors.New(err.Error())
	}
	return nil
}

type KVStoreData struct {
	magicNumber  uint16 // MagicNumber
	keyLength    uint32 // Key长度
	valueLength  uint32 // Value长度
	keyContent   []byte // Key内容
	valueContent []byte // Value内容
}

func NewKVStoreData(key []byte, value []byte) *KVStoreData {
	return &KVStoreData{
		magicNumber:  KVSTORE_MAGIC_NUM,
		keyLength:    uint32(len(key)),
		valueLength:  uint32(len(value)),
		keyContent:   key,
		valueContent: value,
	}
}

func NewKVStoreDataFromBinary(data []byte) (kvStoreData *KVStoreData, err error) {
	if len(data) < 10 {
		return nil, errInvalidLength
	}
	var magicNumber uint16
	if magicNumber = binary.BigEndian.Uint16(data[0:2]); magicNumber != KVSTORE_MAGIC_NUM {
		return nil, errIllegalMagicNumber
	}
	r := &KVStoreData{
		magicNumber:  KVSTORE_MAGIC_NUM,
	}
	r.magicNumber = magicNumber
	r.keyLength = binary.BigEndian.Uint32(data[2:6])
	r.valueLength = binary.BigEndian.Uint32(data[6:10])
	if uint32(len(data)) < 10+r.keyLength+r.valueLength {
		return nil, errInvalidLength
	}
	r.keyContent = make([]byte, r.keyLength)
	copy(r.keyContent, data[10:10+r.keyLength])
	r.valueContent = make([]byte, r.valueLength)
	copy(r.valueContent, data[10+r.keyLength:10+r.keyLength+r.valueLength])
	return r, nil
}
func (r KVStoreData) KeyLength() uint32 {
	return r.keyLength
}

func (r KVStoreData) Key() string {
	return string(r.keyContent)
}

func (r KVStoreData) ValueOffset() uint32 {
	return 10 + r.keyLength
}

func (r KVStoreData) ValueLength() uint32 {
	return r.valueLength
}

func (r KVStoreData) Value() []byte {
	return r.valueContent
}

func (r KVStoreData) Size() uint32 {
	return 10 + r.keyLength + r.valueLength
}

func (r KVStoreData) MarshalBinary() (data []byte, err error) {
	if r.magicNumber != KVSTORE_MAGIC_NUM {
		return nil, errIllegalMagicNumber
	}
	var buf *bytes.Buffer = bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	buf.Reset()
	if err = binary.Write(buf, binary.BigEndian, r.magicNumber); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, r.keyLength); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, r.valueLength); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, r.keyContent); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, r.valueContent); err != nil {
		return
	}
	data = make([]byte, buf.Len())
	copy(data, buf.Bytes())
	return
}
