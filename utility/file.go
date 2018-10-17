package utility

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/klauspost/crc32"
)

var (
	errGetCachedFile        = "ltick utility: get cached file error"
	errGetCachedFileContent = "ltick utility: get cached file content error"
	errCreateTemporaryFile  = "ltick utility: create temporary file error"
)

func Md5File(f *os.File) (string, error) {
	md5hash := md5.New()
	if _, err := io.Copy(md5hash, f); err != nil {
		return "", err
	}
	return string(md5hash.Sum(nil)), nil
}

//获取给定byte的MD5
func Md5Byte(text []byte) string {
	hasher := md5.New()
	hasher.Write(text)
	return hex.EncodeToString(hasher.Sum(nil))
}

func GetCachedFile(filePath string) (file *os.File, err error) {
	fileExtension := path.Ext(filePath)
	cachedFilePath := strings.Replace(filePath, fileExtension, "", -1) + ".cached" + fileExtension
	_, err = os.Stat(cachedFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = NewFile(cachedFilePath, 0644, bytes.NewReader([]byte{}), 0)
			if err != nil {
				return nil, errors.New(errGetCachedFile + ": " + err.Error())
			}
		} else {
			return nil, errors.New(errGetCachedFile + ": " + err.Error())
		}
	} else {
		file, err = os.OpenFile(cachedFilePath, os.O_RDWR, 0644)
		if err != nil {
			return nil, errors.New(errGetCachedFile + ": " + err.Error())
		}
	}
	return file, err
}
func NewFile(filePath string, perm os.FileMode, payload io.Reader, sizes ...int64) (file *os.File, err error) {
	fileDir := filepath.Dir(filePath)
	if _, err = os.Stat(fileDir); os.IsNotExist(err) {
		os.MkdirAll(fileDir, 0775)
	}
	file, err = os.OpenFile(filePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, perm)
	if err != nil {
		return nil, errors.New(errCreateTemporaryFile + ": " + err.Error())
	}
	_, err = file.Seek(0, os.SEEK_SET)
	if err != nil {
		file.Close()
		return nil, errors.New(errCreateTemporaryFile + ": " + err.Error())
	}
	if len(sizes) > 0 {
		size := sizes[0]
		_, err = io.CopyN(file, payload, size)
		if err != nil {
			file.Close()
			return nil, errors.New(errCreateTemporaryFile + ": " + err.Error())
		}
	} else {
		_, err = io.Copy(file, payload)
		if err != nil {
			file.Close()
			return nil, errors.New(errCreateTemporaryFile + ": " + err.Error())
		}
	}
	return file, nil
}

func NewTemporaryFile(temporaryPath string, prefix string, payload io.Reader, sizes ...int64) (file *os.File, fileSize int64, err error) {
	if _, err = os.Stat(temporaryPath); os.IsNotExist(err) {
		os.MkdirAll(temporaryPath, 0775)
	}
	file, err = ioutil.TempFile(temporaryPath, prefix)
	if err != nil {
		return nil, 0, errors.New(errCreateTemporaryFile + ": " + err.Error())
	}
	_, err = file.Seek(0, os.SEEK_SET)
	if err != nil {
		file.Close()
		return nil, 0, errors.New(errCreateTemporaryFile + ": " + err.Error())
	}
	if len(sizes) > 0 {
		size := sizes[0]
		fileSize, err = io.CopyN(file, payload, size)
		if err != nil {
			file.Close()
			return nil, 0, errors.New(errCreateTemporaryFile + ": " + err.Error())
		}
	} else {
		fileSize, err = io.Copy(file, payload)
		if err != nil {
			file.Close()
			return nil, 0, errors.New(errCreateTemporaryFile + ": " + err.Error())
		}
	}
	return file, fileSize, nil
}

func Checksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}
