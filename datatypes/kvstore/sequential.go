package kvstore

import (
	"bufio"
	"encoding"
	"encoding/binary"
	"os"
)

/*
type KVStoreReader interface {
	ReadOne(encoding.BinaryUnmarshaler) error
	Offset() int
	Close() error
}
*/

type KVStoreReader struct {
	file   *os.File
	br     *bufio.Reader
	offset int
}

func NewKVStoreReader(filename string) (fr *KVStoreReader, err error) {
	var file *os.File
	if file, err = os.OpenFile(filename, os.O_RDONLY, os.FileMode(0644)); err != nil {
		return
	}
	fr = &KVStoreReader{
		file:   file,
		br:     bufio.NewReader(file),
		offset: 0,
	}
	return
}

func (fr *KVStoreReader) readOne() (data []byte, err error) {
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

func (fr *KVStoreReader) ReadOne(item encoding.BinaryUnmarshaler) (err error) {
	var data []byte
	if data, err = fr.readOne(); err != nil {
		return
	}
	return item.UnmarshalBinary(data)
}

func (fr *KVStoreReader) Offset() int {
	return fr.offset
}

func (fr *KVStoreReader) Close() error {
	return fr.file.Close()
}
