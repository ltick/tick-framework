package lru

import (
	"bufio"
	"encoding"
	"os"
)

type File struct {
	file *os.File

	// Reader
	br *bufio.Reader

	// Writer
	bw *bufio.Writer
}

type EncodeCloser interface {
	Encode(bm encoding.BinaryMarshaler) error
	Truncate() error
	Close() error
}

func NewWriteFile(name string) (_ EncodeCloser, err error) {
	var (
		file *os.File
		f    *File
	)
	if file, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE, os.FileMode(0644)); err != nil {
		return
	}
	if _, err = file.Seek(0, os.SEEK_END); err != nil {
		return
	}
	f = &File{
		file: file,
		bw:   bufio.NewWriter(file),
	}
	return f, nil
}

func (f *File) Write(b []byte) (n int, err error) {
	if n, err = f.bw.Write(b); err != nil {
		return
	}
	if err = f.bw.Flush(); err != nil {
		return
	}
	return
}

func (f *File) Encode(bm encoding.BinaryMarshaler) (err error) {
	var data []byte
	if data, err = bm.MarshalBinary(); err != nil {
		return
	}
	if _, err = f.Write(data); err != nil {
		return
	}
	return
}

func (f *File) Truncate() (err error) {
	if err = f.file.Truncate(0); err != nil {
		return
	}
	if _, err = f.file.Seek(0, os.SEEK_SET); err != nil {
		return
	}
	f.bw.Reset(f.file)
	return
}

func (f *File) Close() error {
	return f.file.Close()
}
