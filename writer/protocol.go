package writer

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strconv"
	"sync"

	"gopkg.in/h2non/filetype.v1"

	mediaResource "nebula/media/app/media/resource"
)

const (
	vary            = "Vary"
	acceptEncoding  = "Accept-Encoding"
	contentEncoding = "Content-Encoding"
	contentType     = "Content-Type"
	contentLength   = "Content-Length"
)

var (
	errCompress = "storage response writer: compress error"
)

type Data struct {
	Code    string      `json:"code" xml:"code"`
	Status  int         `json:"status" xml:"status"`
	Message string      `json:"message,omitempty" xml:"message,omitempty"`
	Data    interface{} `json:"data,omitempty" xml:"data,omitempty"`
}

