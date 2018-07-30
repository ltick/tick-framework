package kvstore

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	magicNumber  uint16 // MagicNumber
	keyLength    uint32 // Key长度
	valueLength  uint32 // Value长度
	keyContent   []byte // Key内容
	valueContent []byte // Value内容
}

func NewKVStore(key []byte, value []byte) KVStore {
	return KVStore{
		magicNumber:  KVSTORE_MAGIC_NUM,
		keyLength:    uint32(len(key)),
		valueLength:  uint32(len(value)),
		keyContent:   key,
		valueContent: value,
	}
}

func (r KVStore) KeyLength() uint32 {
	return r.keyLength
}

func (r KVStore) Key() string {
	return string(r.keyContent)
}

func (r KVStore) ValueOffset() uint32 {
	return 10 + r.keyLength
}

func (r KVStore) ValueLength() uint32 {
	return r.valueLength
}

func (r KVStore) Value() []byte {
	return r.valueContent
}

func (r KVStore) Size() uint32 {
	return 10 + r.keyLength + r.valueLength
}

func (r KVStore) MarshalBinary() (data []byte, err error) {
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

func (r *KVStore) UnmarshalBinary(data []byte) (err error) {
	if len(data) < 10 {
		return errInvalidLength
	}
	var magicNumber uint16
	if magicNumber = binary.BigEndian.Uint16(data[0:2]); magicNumber != KVSTORE_MAGIC_NUM {
		return errIllegalMagicNumber
	}
	r.magicNumber = magicNumber
	r.keyLength = binary.BigEndian.Uint32(data[2:6])
	r.valueLength = binary.BigEndian.Uint32(data[6:10])
	if uint32(len(data)) < 10+r.keyLength+r.valueLength {
		return errInvalidLength
	}
	r.keyContent = make([]byte, r.keyLength)
	copy(r.keyContent, data[10:10+r.keyLength])
	r.valueContent = make([]byte, r.valueLength)
	copy(r.valueContent, data[10+r.keyLength:10+r.keyLength+r.valueLength])
	return
}
