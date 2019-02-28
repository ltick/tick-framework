package utility

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"io"
)

func HexSha256(data string) string {
	var h hash.Hash = sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func Base64Md5ReadSeeker(readSeeker io.ReadSeeker) (_ string, err error) {
	var h hash.Hash = md5.New()
	if _, err = readSeeker.Seek(0, io.SeekStart); err != nil {
		return
	}
	if _, err = io.Copy(h, readSeeker); err != nil {
		return
	}
	if _, err = readSeeker.Seek(0, io.SeekStart); err != nil {
		return
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
