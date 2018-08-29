package ltick

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

	"github.com/ltick/tick-framework/writer"
	"github.com/ltick/tick-routing"
	"fmt"
	"encoding/json"
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
func (e *Engine) NewResponseWriter(c *routing.Context, w routing.DataWriter)  (r *Response) {
	r = &Response{}
	r.SetDataWriter(w)
	return r
}
func (e *Engine) NewResponse(c *routing.Context, w routing.DataWriter)  (r *Response) {
	r = &Response{}
	r.SetDataWriter(&ResponseWriter{})
	return r
}

// Response wraps an http.ResponseWriter and implements its interface to be used
// by an HTTP handler to construct an HTTP response.
// See [http.ResponseWriter](https://golang.org/pkg/net/http/#ResponseWriter)
type ResponseWriter struct {

}

func (rw *ResponseWriter) SetHeader(w http.ResponseWriter) {

}

func (rw *ResponseWriter) Write(w http.ResponseWriter, data interface{}) (err error) {
	switch data.(type) {
	case []byte:
		byte := data.([]byte)
		_, err = w.Write(byte)
	case string:
		byte := []byte(data.(string))
		_, err = w.Write(byte)
	case *writer.ErrorData:
		errorData, ok := data.(*writer.ErrorData)
		if !ok {
			return routing.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("get audio: data type error"))
		}
		errorDataBody, err := json.Marshal(errorData)
		if err != nil {
			return routing.NewHTTPError(errorData.StatusCode(), errorData.ErrorCode()+":"+errorData.Error())
		}
		rw.SetHeader(w)
		return routing.NewHTTPError(errorData.StatusCode(), string(errorDataBody))
	default:
		if data != nil {
			_, err = fmt.Fprint(w, data)
		}
	}
	if err != nil {
		if ConnectionResetByPeer(err) || Timeout(err) || NetworkUnreachable(err) {
			return  routing.NewHTTPError(499, "Response write error: "+err.Error())
		}
		return  routing.NewHTTPError(http.StatusGatewayTimeout, "Response write error: "+err.Error())
	}
	return err
}

type Response struct {
	httpResponseWriter http.ResponseWriter
	responseWriter routing.DataWriter
	status int
	wrote  bool
}

func (r *Response) reset(w http.ResponseWriter) {
	r.httpResponseWriter = w
	r.responseWriter = &ResponseWriter{}
	r.status = 0
	r.wrote = false
}

func (r *Response) SetDataWriter(w routing.DataWriter) {
	r.responseWriter = w
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

// WriteHeader sends an HTTP response header with status code.
// If WriteHeader is not called explicitly, the first call to Write
// will trigger an implicit WriteHeader(http.StatusOK).
// Thus explicit calls to WriteHeader are mainly used to
// send error codes.
func (r *Response) WriteHeader(status int) {
	if r.wrote {
		r.wroteCallback()
		return
	}
	r.status = status

	r.httpResponseWriter.WriteHeader(status)
	if r.responseWriter != nil {
		r.responseWriter.SetHeader(r.httpResponseWriter)
	}
	r.wrote = true
}

// Write writes the data to the connection as part of an HTTP reply.
// If WriteHeader has not yet been called, Write calls WriteHeader(http.StatusOK)
// before writing the data.  If the Header does not contain a
// Content-Type line, Write adds a Content-Type set to the result of passing
// the initial 512 bytes of written data to DetectContentType.
func (r *Response) Write(data interface{}) (n int, err error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	if r.responseWriter != nil {
		n, err = r.responseWriter.Write(r.httpResponseWriter, data)
	} else {
		b := data.([]byte)
		n, err = r.httpResponseWriter.Write(b)
	}
	return n, err
}

func (r *Response) NewResponse(c *routing.Context, code string, data interface{}, messages ...string) error {
	responseData := r.NewResponseData(c, code, data, messages...)
	_, err := r.Write(responseData)
	if err != nil {
		if ConnectionResetByPeer(err) || Timeout(err) || NetworkUnreachable(err) {
			return routing.NewHTTPError(499, "Response write error: "+err.Error())
		}
		return routing.NewHTTPError(http.StatusInternalServerError, "Response write error: "+err.Error())
	}
	return nil
}
func (r *Response) NewErrorResponse(c *routing.Context, code string, messages ...string) error {
	responseData := r.NewErrorResponseData(c, code, messages...)
	_, err := r.Write(responseData)
	if err != nil {
		if ConnectionResetByPeer(err) || Timeout(err) || NetworkUnreachable(err) {
			return routing.NewHTTPError(499, "Response write error: "+err.Error())
		}
		return routing.NewHTTPError(http.StatusInternalServerError, "Response write error: "+err.Error())
	}
	return nil
}
func (r *Response) NewResponseData(c *routing.Context, code string, data interface{}, messages ...string) *writer.Data {
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
	return &writer.Data{
		Code:    code,
		Status:  config["status"].(int),
		Message: message,
		Data:    data,
	}
}
func (r *Response) NewErrorResponseData(c *routing.Context, code string, messages ...string) *writer.ErrorData {
	return &writer.ErrorData{
		Data: r.NewResponseData(c, code, nil, messages...),
	}
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
		Log().Warningf("multiple response.WriteHeader calls\n[TRACE]\n%s\n", stack)
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
