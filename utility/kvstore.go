package utility

import (
	"bytes"
	"encoding/binary"
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

type KvstoreData struct {
	magicNumber  uint16 // MagicNumber
	keyLength    uint32 // Key长度
	valueLength  uint32 // Value长度
	keyContent   []byte // Key内容
	valueContent []byte // Value内容
}

func NewKvstoreData(key []byte, value []byte) *KvstoreData {
	return &KvstoreData{
		magicNumber:  KVSTORE_MAGIC_NUM,
		keyLength:    uint32(len(key)),
		valueLength:  uint32(len(value)),
		keyContent:   key,
		valueContent: value,
	}
}

func (kvData *KvstoreData) KeyLength() uint32 {
	return kvData.keyLength
}

func (kvData *KvstoreData) Key() string {
	return string(kvData.keyContent)
}

func (kvData *KvstoreData) ValueOffset() uint32 {
	return 10 + kvData.keyLength
}

func (kvData *KvstoreData) ValueLength() uint32 {
	return kvData.valueLength
}

func (kvData *KvstoreData) Value() []byte {
	return kvData.valueContent
}

func (kvData *KvstoreData) Size() uint32 {
	return 10 + kvData.keyLength + kvData.valueLength
}

func (kvData *KvstoreData) MarshalBinary() (data []byte, err error) {
	if kvData.magicNumber != KVSTORE_MAGIC_NUM {
		return nil, errIllegalMagicNumber
	}
	var buf *bytes.Buffer = bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	buf.Reset()
	if err = binary.Write(buf, binary.BigEndian, kvData.magicNumber); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, kvData.keyLength); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, kvData.valueLength); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, kvData.keyContent); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, kvData.valueContent); err != nil {
		return
	}
	data = make([]byte, buf.Len())
	copy(data, buf.Bytes())
	return
}

func (kvData *KvstoreData) UnmarshalBinary(data []byte) (err error) {
	if len(data) < 10 {
		return errInvalidLength
	}
	var magicNumber uint16
	if magicNumber = binary.BigEndian.Uint16(data[0:2]); magicNumber != KVSTORE_MAGIC_NUM {
		return errIllegalMagicNumber
	}
	kvData.magicNumber = magicNumber
	kvData.keyLength = binary.BigEndian.Uint32(data[2:6])
	kvData.valueLength = binary.BigEndian.Uint32(data[6:10])
	if uint32(len(data)) < 10+kvData.keyLength+kvData.valueLength {
		return errInvalidLength
	}
	kvData.keyContent = make([]byte, kvData.keyLength)
	copy(kvData.keyContent, data[10:10+kvData.keyLength])
	kvData.valueContent = make([]byte, kvData.valueLength)
	copy(kvData.valueContent, data[10+kvData.keyLength:10+kvData.keyLength+kvData.valueLength])
	return
}

type Kvstore struct {
	reader io.ReadSeeker
	writer io.WriteSeeker
}

func NewKvstore(reader io.ReadSeeker, writer io.WriteSeeker) *Kvstore {
	return &Kvstore{
		reader: reader,
		writer: writer,
	}
}

func (kv *Kvstore) get(offset int64) (data []byte, err error) {
	var (
		overheadLength uint32 = 10
		buf            []byte = make([]byte, overheadLength)
		keyLength      uint32
		valueLength    uint32
	)
	if _, err = kv.reader.Seek(offset, os.SEEK_SET); err != nil {
		return
	}
	if _, err = kv.reader.Read(buf); err != nil {
		return
	}
	keyLength = binary.BigEndian.Uint32(buf[2:6])
	valueLength = binary.BigEndian.Uint32(buf[6:10])
	data = make([]byte, overheadLength+keyLength+valueLength)
	copy(data, buf)
	var (
		bufferBytes []byte
		readN       int
	)
	for bufferBytes = data[overheadLength:]; len(bufferBytes) > 0; bufferBytes = bufferBytes[readN:] {
		if readN, err = kv.reader.Read(bufferBytes); err != nil {
			return
		}
	}
	return
}

func (kv *Kvstore) Get(offset int64) (kvData *KvstoreData, err error) {
	var data []byte
	if data, err = kv.get(offset); err != nil {
		return
	}
	kvData = &KvstoreData{}
	if err = kvData.UnmarshalBinary(data); err != nil {
		return
	}
	return
}

func (kv *Kvstore) Set(kvData *KvstoreData) (err error) {
	var data []byte
	if data, err = kvData.MarshalBinary(); err != nil {
		return
	}
	if _, err = kv.writer.Seek(0, os.SEEK_END); err != nil {
		return
	}
	if _, err = kv.writer.Write(data); err != nil {
		return
	}
	return
}

type nopStore struct {
}

func (nop *nopStore) Read(p []byte) (n int, err error) {
	return
}
func (nop *nopStore) Write(p []byte) (n int, err error) {
	return
}
func (nop *nopStore) Seek(offset int64, whence int) (ret int64, err error) {
	return
}
func (nop *nopStore) Close() (err error) {
	return
}

var NopStore *nopStore = &nopStore{}