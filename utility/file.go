package utility

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/crc32"
	"fmt"
)

var (
	errGetCacheFile        = "ltick utility: get cache file error"
	errGetCacheFileContent = "ltick utility: get cache file content error"
	errCreateTemporaryFile  = "ltick utility: create temporary file error"
)

const BUFFERSIZE  = 1024

func CopyFile(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	buf := make([]byte, BUFFERSIZE)
	var cn int64
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return 0, err
		}
		cn += int64(n)
		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return 0, err
		}
	}
	return cn, nil
}

func GetCacheFile(cacheFile string, cacheFiles ...string) (file *os.File, err error) {
	fileExtension := filepath.Ext(cacheFile)
	cachedFilePath := strings.Replace(cacheFile, fileExtension, "", -1) + ".cached" + fileExtension
	if len(cacheFiles) >0 {
		cachedFilePath = strings.Replace(cachedFilePath, filepath.Dir(cachedFilePath), cacheFiles[0], 1)
	}
	_, err = os.Stat(cachedFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = NewFile(cachedFilePath, 0644, bytes.NewReader([]byte{}), 0)
			if err != nil {
				return nil, errors.New(errGetCacheFile + ": " + err.Error())
			}
		} else {
			return nil, errors.New(errGetCacheFile + ": " + err.Error())
		}
	} else {
		file, err = os.OpenFile(cachedFilePath, os.O_RDWR, 0644)
		if err != nil {
			return nil, errors.New(errGetCacheFile + ": " + err.Error())
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

// SelfPath gets compiled executable file absolute path.
func SelfPath() string {
	path, _ := filepath.Abs(os.Args[0])
	return path
}

// SelfDir gets compiled executable file directory.
func SelfDir() string {
	return filepath.Dir(SelfPath())
}

// RelPath gets relative path.
func RelPath(targpath string) string {
	basepath, _ := filepath.Abs("./")
	rel, _ := filepath.Rel(basepath, targpath)
	return strings.Replace(rel, `\`, `/`, -1)
}

// FileExists reports whether the named file or directory exists.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		return !os.IsNotExist(err)
	}
	return true
}

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
