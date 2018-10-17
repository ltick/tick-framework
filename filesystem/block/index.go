package block

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
)

var (
	errIndexInvalidLength error = fmt.Errorf("index: invalid length")
)

var (
	bufPool sync.Pool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}
)

type Index struct {
	filenameLength  uint32 // 文件名长度
	filenameContent []byte // 文件名内容
	offset          uint64 // 索引位置
	length          uint32 // 内容大小
}

func (idx *Index) Same(other *Index) bool {
	if idx.filenameLength == other.filenameLength &&
		idx.offset == other.offset &&
		idx.length == other.length &&
		bytes.Equal(idx.filenameContent, other.filenameContent) {
		return true
	}
	return false
}

func NewIndex(filename string, offset uint64, length uint32) (idx *Index) {
	return &Index{
		filenameLength:  uint32(len(filename)),
		filenameContent: []byte(filename),
		offset:          offset,
		length:          length,
	}
}

func NewDelIndex() (idx *Index) { // 文件名为空, 表示删除
	return NewIndex("", 0, 0)
}

func (idx *Index) IsDel() bool {
	return idx.filenameLength == 0
}

func (idx *Index) Filename() string {
	return string(idx.filenameContent)
}

func (idx *Index) MarshalBinary() (data []byte, err error) {
	var buf *bytes.Buffer = bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	buf.Reset()
	if err = binary.Write(buf, binary.BigEndian, idx.filenameLength); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, idx.filenameContent); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, idx.offset); err != nil {
		return
	}
	if err = binary.Write(buf, binary.BigEndian, idx.length); err != nil {
		return
	}
	data = make([]byte, buf.Len())
	copy(data, buf.Bytes())
	return
}

func (idx *Index) UnmarshalBinary(data []byte) (err error) {
	if len(data) < 4 {
		return errIndexInvalidLength
	}
	idx.filenameLength = binary.BigEndian.Uint32(data[0:4])
	if uint32(len(data)) < 4+idx.filenameLength+8+4 {
		return errIndexInvalidLength
	}
	idx.filenameContent = make([]byte, idx.filenameLength)
	copy(idx.filenameContent, data[4:4+idx.filenameLength])
	idx.offset = binary.BigEndian.Uint64(data[4+idx.filenameLength : 12+idx.filenameLength])
	idx.length = binary.BigEndian.Uint32(data[12+idx.filenameLength : 16+idx.filenameLength])
	return
}
