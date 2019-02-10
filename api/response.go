package api

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"syscall"

	"github.com/ltick/tick-routing"
	"github.com/ltick/tick-routing/content"
)

var responseOptions = map[string]map[string]interface{}{
	"Success": map[string]interface{}{
		"status":  http.StatusOK,
		"message": "Success",
	},
	"InvalidURI": map[string]interface{}{
		"status":  http.StatusBadRequest,
		"message": "Couldn't parse the specified URI.",
	},
	"BadRequest": map[string]interface{}{
		"status":  http.StatusBadRequest,
		"message": "Bad request",
	},
	"EntityTooLarge": map[string]interface{}{
		"status":  http.StatusBadRequest,
		"message": "Your proposed upload exceeds the maximum allowed object size.",
	},
	"AccessDenied": map[string]interface{}{
		"status":  http.StatusForbidden,
		"message": "Access denied",
	},
	"NotFound": map[string]interface{}{
		"status":  http.StatusNotFound,
		"message": "The specified record does not exist",
	},
	"RequestTimeout": map[string]interface{}{
		"status":  http.StatusRequestTimeout,
		"message": "Your socket connection to the server was not read from or written to within the timeout period",
	},
	"InternalError": map[string]interface{}{
		"status":  http.StatusInternalServerError,
		"message": "We encountered an internal error. Please try again",
	},
	"BadGateway": map[string]interface{}{
		"status":  http.StatusBadGateway,
		"message": "Bad gateway",
	},
	"SlowDown": map[string]interface{}{
		"status":  http.StatusServiceUnavailable,
		"message": "Reduce your request rate",
	},
	"ServiceUnavailable": map[string]interface{}{
		"status":  http.StatusServiceUnavailable,
		"message": "Service unavailable",
	},
	"RequestedRangeNotSatisfiable": map[string]interface{}{
		"status":  http.StatusRequestedRangeNotSatisfiable,
		"message": http.StatusText(http.StatusRequestedRangeNotSatisfiable),
	},
}

// ResponseData is commonly used to return JSON format response.
type ResponseData struct {
	Code    string      `json:"code" xml:"code"` // the status code of the business process (required)
	Status  int         `json:"status,omitempty" xml:"status,omitempty"`
	Message string      `json:"message,omitempty" xml:"message,omitempty"`
	Data    interface{} `json:"data,omitempty" xml:"data,omitempty"`
}

func (this *ResponseData) GetMessage() string {
	return this.Message
}
func (this *ResponseData) GetStatus() int {
	return this.Status
}
func (this *ResponseData) GetCode() string {
	return this.Code
}
func (this *ResponseData) GetData() interface{} {
	return this.Data
}

func NewResponseData(code string, data interface{}, messages ...string) *ResponseData {
	config := make(map[string]interface{})
	responseConfig, ok := responseOptions[code]
	if ok {
		config = responseConfig
	} else {
		config["message"] = "error code not exists"
		config["status"] = http.StatusInternalServerError
	}
	message := config["message"].(string)
	if len(messages) > 0 {
		message = messages[0]
	}
	status, ok := config["status"].(int)
	if !ok {
		status = http.StatusOK
	}
	return &ResponseData{
		Status:  status,
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func RegisterResponseOption(code string, status int, message string) {
	responseOptions[code] = map[string]interface{}{
		"status":  status,
		"message": message,
	}
}

func NewResponse(rw http.ResponseWriter, w ...routing.DataWriter) (r *Response) {
	r = &Response{
		httpResponseWriter: rw,
	}
	if len(w) > 0 {
		r.SetDataWriter(w[0])
	} else {
		r.SetDataWriter(&content.HTMLDataWriter{})
	}
	return r
}

type Response struct {
	httpResponseWriter http.ResponseWriter
	responseWriter     routing.DataWriter
	status             int
	wrote              bool
}

func (r *Response) reset(w http.ResponseWriter) {
	r.httpResponseWriter = w
	r.responseWriter = nil
	r.status = 0
	r.wrote = false
}

func (r *Response) SetDataWriter(w routing.DataWriter) *Response {
	r.responseWriter = w
	return r
}

// Header returns the header map that will be sent by
// WriteHeader. Changing the header after a call to
// WriteHeader (or Write) has no effect unless the modified
// headers were declared as trailers by setting the
// "Trailer" header before the call to WriteHeader (see example).
// To suppress implicit response headers, set their value to nil.
func (r *Response) Header() http.Header {
	return r.httpResponseWriter.Header()
}

// Write writes the data to the connection as part of an HTTP reply.
// If WriteHeader has not yet been called, Write calls WriteHeader(http.StatusOK)
// before writing the data.  If the Header does not contain a
// Content-Type line, Write adds a Content-Type set to the result of passing
// the initial 512 bytes of written data to DetectContentType.
func (r *Response) Write(data interface{}) (n int, err error) {
	if r.wrote == true {
		return 0, nil
	}
	if r.responseWriter != nil {
		n, err = r.responseWriter.Write(r.httpResponseWriter, data)
	} else {
		b := data.([]byte)
		n, err = r.httpResponseWriter.Write(b)
	}
	r.wrote = true
	if err != nil {
		if ConnectionResetByPeer(err) || Timeout(err) || NetworkUnreachable(err) {
			return 0, routing.NewHTTPError(499, "Response write error: "+err.Error())
		}
		return 0, routing.NewHTTPError(http.StatusInternalServerError, "Response write error: "+err.Error())
	}
	return n, err
}

// AddCookie adds a Set-Cookie header.
// The provided cookie must have a valid Name. Invalid cookies may be
// silently dropped.
func (r *Response) AddCookie(cookie *http.Cookie) {
	r.Header().Add(HeaderSetCookie, cookie.String())
}

// SetCookie sets a Set-Cookie header.
func (r *Response) SetCookie(cookie *http.Cookie) {
	r.Header().Set(HeaderSetCookie, cookie.String())
}

// DelCookie sets Set-Cookie header.
func (r *Response) DelCookie() {
	r.Header().Del(HeaderSetCookie)
}

// Copy is here to optimize copying from an *os.File regular file
// to a *net.TCPConn with sendfile.
func (r *Response) Copy(src io.Reader) (int64, error) {
	if rf, ok := r.httpResponseWriter.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(src)
		return n, err
	}
	var buf = make([]byte, 32*1024)
	var n int64
	var err error
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := r.httpResponseWriter.Write(buf[0:nr])
			if nw > 0 {
				n += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	if err != nil {
		if ConnectionResetByPeer(err) || Timeout(err) || NetworkUnreachable(err) {
			return 0, routing.NewHTTPError(499, "Response write error: "+err.Error())
		}
		return 0, routing.NewHTTPError(http.StatusGatewayTimeout, "Response write error: "+err.Error())
	}
	return n, err
}

// Flush implements the http.Flusher interface to allow an HTTP handler to flush
// buffered data to the client.
func (r *Response) Flush() {
	if f, ok := r.httpResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack implements the http.Hijacker interface to allow an HTTP handler to
// take over the connection.
func (r *Response) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := r.httpResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("webserver doesn't support Hijack")
}

// CloseNotify implements the http.CloseNotifier interface to allow detecting
// when the underlying connection has gone away.
// This mechanism can be used to cancel long operations on the server if the
// client has disconnected before the response is ready.
func (r *Response) CloseNotify() <-chan bool {
	if cn, ok := r.httpResponseWriter.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}
	return nil
}

// Wrote returns whether the response has been submitted or not.
func (r *Response) Wrote() bool {
	return r.wrote
}

// Status returns the HTTP status code of the response.
func (r *Response) Status() int {
	return r.status
}

func (r *Response) wroteCallback() {
	if r.status == 200 {
		line := []byte("\n")
		e := []byte("\ngoroutine ")
		stack := make([]byte, 2<<10) //2KB
		runtime.Stack(stack, true)
		start := bytes.Index(stack, line) + 1
		stack = stack[start:]
		end := bytes.LastIndex(stack, line)
		if end != -1 {
			stack = stack[:end]
		}
		end = bytes.Index(stack, e)
		if end != -1 {
			stack = stack[:end]
		}
		stack = bytes.TrimRight(stack, "\n")
	}
}

func ConnectionResetByPeer(err error) bool {
	return strings.Contains(err.Error(), syscall.ECONNRESET.Error())
}

func Timeout(err error) bool {
	return strings.Contains(err.Error(), "i/o timeout")
}

func NetworkUnreachable(err error) bool {
	return strings.Contains(err.Error(), "network is unreachable")
}
