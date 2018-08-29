package writer

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const (
	// DefaultQValue is the default qvalue to assign to an encoding if no explicit qvalue is set.
	// This is actually kind of ambiguous in RFC 2616, so hopefully it's correct.
	// The examples seem to indicate that it is.
	DefaultQValue = 1.0

	// 1500 bytes is the MTU size for the internet since that is the largest size allowed at the network layer.
	// If you take a file that is 1300 bytes and compress it to 800 bytes, it’s still transmitted in that same 1500 byte packet regardless, so you’ve gained nothing.
	// That being the case, you should restrict the gzip compression to files with a size greater than a single packet, 1400 bytes (1.4KB) is a safe value.
	DefaultMinSize = 1400
)

type codings map[string]float64

// gzipWriterPools stores a sync.Pool for each compression level for reuse of
// gzip.Writers. Use poolIndex to covert a compression level to an index into
// gzipWriterPools.
var GzipWriterPools [gzip.BestCompression - gzip.BestSpeed + 2]*sync.Pool
var GzipBufferPool sync.Pool = sync.Pool{New: func() interface{} {
	return new(bytes.Buffer)
}}

// parseEncodings attempts to parse a list of codings, per RFC 2616, as might
// appear in an Accept-Encoding header. It returns a map of content-codings to
// quality values, and an error containing the errors encountered. It's probably
// safe to ignore those, because silently ignoring errors is how the internet
// works.
//
// See: http://tools.ietf.org/html/rfc2616#section-14.3.
func parseEncodings(s string) (codings, error) {
	c := make(codings)
	var e []string

	for _, ss := range strings.Split(s, ",") {
		coding, qvalue, err := parseCoding(ss)

		if err != nil {
			e = append(e, err.Error())
		} else {
			c[coding] = qvalue
		}
	}

	if len(e) > 0 {
		return c, fmt.Errorf("errors while parsing encodings: %s", strings.Join(e, ", "))
	}

	return c, nil
}

// Used for functional configuration.
type compressConfig struct {
	minSize              int
	level                int
	acceptedContentTypes []parsedContentType
}

func (c *compressConfig) validate() error {
	if c.level != gzip.DefaultCompression && (c.level < gzip.BestSpeed || c.level > gzip.BestCompression) {
		return fmt.Errorf("invalid compression level requested: %d", c.level)
	}

	if c.minSize < 0 {
		return fmt.Errorf("minimum size must be more than zero")
	}

	return nil
}

// parseCoding parses a single conding (content-coding with an optional qvalue),
// as might appear in an Accept-Encoding header. It attempts to forgive minor
// formatting errors.
func parseCoding(s string) (coding string, qvalue float64, err error) {
	for n, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		qvalue = DefaultQValue

		if n == 0 {
			coding = strings.ToLower(part)
		} else if strings.HasPrefix(part, "q=") {
			qvalue, err = strconv.ParseFloat(strings.TrimPrefix(part, "q="), 64)

			if qvalue < 0.0 {
				qvalue = 0.0
			} else if qvalue > 1.0 {
				qvalue = 1.0
			}
		}
	}

	if coding == "" {
		err = fmt.Errorf("empty content-coding")
	}

	return
}

// acceptsGzip returns true if the given HTTP request indicates that it will
// accept a gzipped response.
func acceptEncodingGzip(r *http.Request) bool {
	acceptedEncodings, _ := parseEncodings(r.Header.Get(acceptEncoding))
	return acceptedEncodings["gzip"] > 0.0
}

// Parsed representation of one of the inputs to ContentTypes.
// See https://golang.org/pkg/mime/#ParseMediaType
type parsedContentType struct {
	mediaType string
	params    map[string]string
}

// equals returns whether this content type matches another content type.
func (pct parsedContentType) equals(mediaType string, params map[string]string) bool {
	if pct.mediaType != mediaType {
		return false
	}
	// if pct has no params, don't care about other's params
	if len(pct.params) == 0 {
		return true
	}

	// if pct has any params, they must be identical to other's.
	if len(pct.params) != len(params) {
		return false
	}
	for k, v := range pct.params {
		if w, ok := params[k]; !ok || v != w {
			return false
		}
	}
	return true
}

func handleContentType(contentTypes []parsedContentType, w http.ResponseWriter) bool {
	// If contentTypes is empty we handle all content types.
	if contentTypes == nil || len(contentTypes) == 0 {
		return true
	}

	ct := w.Header().Get(contentType)
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}

	for _, c := range contentTypes {
		if c.equals(mediaType, params) {
			return true
		}
	}

	return false
}

func GzipWrite(rw http.ResponseWriter, config *compressConfig, b []byte) (int, error) {
	// If the global writes are bigger than the minSize and we're about to write
	// a response containing a content type we want to handle, enable
	// compression.
	if len(b) >= config.minSize && handleContentType(config.acceptedContentTypes, rw) && rw.Header().Get(contentEncoding) == "" {
		writer := GzipWriterPools[poolIndex(gzip.DefaultCompression)].Get().(*gzip.Writer)
		// GZIP responseWriter is initialized. Use the GZIP responseWriter.
		if writer == nil {
			return 0, errors.New("gzip writer init error")
		}
		buffer := GzipBufferPool.Get().(*bytes.Buffer)
		defer func() {
			// 归还buff
			buffer.Reset()
			GzipBufferPool.Put(buffer)
			// 归还Writer
			GzipWriterPools[poolIndex(gzip.DefaultCompression)].Put(writer)
		}()
		// If content type is not set.
		if _, ok := rw.Header()[contentType]; !ok {
			// It infer it from the uncompressed body.
			rw.Header().Set(contentType, http.DetectContentType(b))
		}
		// Set the GZIP header.
		rw.Header().Set(contentEncoding, "gzip")
		// if the Content-Length is already set, then calls to Write on gzip
		// will fail to set the Content-Length header since its already set
		// See: https://github.com/golang/go/issues/14975.
		rw.Header().Del(contentLength)
		// start gzip
		writer.Reset(rw)
		l, err := writer.Write(b)
		// This should never happen (per io.Writer docs), but if the write didn't
		// accept the entire buffer but returned no specific error, we have no clue
		// what's going on, so abort just to be safe.
		if err != nil {
			return 0, err
		} else if l < len(b) {
			return 0, io.ErrShortWrite
		}
		err = writer.Flush()
		if err != nil {
			return 0, err
		}
		err = writer.Close()
		if err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

func addLevelPool(level int) {
	GzipWriterPools[poolIndex(level)] = &sync.Pool{
		New: func() interface{} {
			// NewWriterLevel only returns error on a bad level, we are guaranteeing
			// that this will be a valid level so it is okay to ignore the returned
			// error.
			buf := new(bytes.Buffer)
			w, _ := gzip.NewWriterLevel(buf, level)
			return w
		},
	}
}

// poolIndex maps a compression level to its index into gzipWriterPools. It
// assumes that level is a valid gzip compression level.
func poolIndex(level int) int {
	// gzip.DefaultCompression == -1, so we need to treat it special.
	if level == gzip.DefaultCompression {
		return gzip.BestCompression - gzip.BestSpeed + 1
	}
	return level - gzip.BestSpeed
}

func init() {
	for i := gzip.BestSpeed; i <= gzip.BestCompression; i++ {
		addLevelPool(i)
	}
	addLevelPool(gzip.DefaultCompression)
}
