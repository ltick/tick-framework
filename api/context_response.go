// Copyright 2016 HenryLee. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/ltick/tick-framework/api/acceptencoder"
	"github.com/ltick/tick-routing"
)

// Wrote returns whether the response has been submitted or not.
func (ctx *Context) Wrote() bool {
	return ctx.Response.Wrote()
}

// Status returns the HTTP status code of the response.
func (ctx *Context) Status() int {
	return ctx.Response.Status()
}

// IsCachable returns boolean of this request is cached.
// HTTP 304 means cached.
func (ctx *Context) IsCachable() bool {
	return ctx.Response.Status() >= 200 && ctx.Response.Status() < 300 || ctx.Response.Status() == 304
}

// IsEmpty returns boolean of this request is empty.
// HTTP 201，204 and 304 means empty.
func (ctx *Context) IsEmpty() bool {
	return ctx.Response.Status() == 201 || ctx.Response.Status() == 204 || ctx.Response.Status() == 304
}

// IsOk returns boolean of this request runs well.
// HTTP 200 means ok.
func (ctx *Context) IsOk() bool {
	return ctx.Response.Status() == 200
}

// IsSuccessful returns boolean of this request runs successfully.
// HTTP 2xx means ok.
func (ctx *Context) IsSuccessful() bool {
	return ctx.Response.Status() >= 200 && ctx.Response.Status() < 300
}

// IsRedirect returns boolean of this request is redirection header.
// HTTP 301,302,307 means redirection.
func (ctx *Context) IsRedirect() bool {
	return ctx.Response.Status() == 301 || ctx.Response.Status() == 302 || ctx.Response.Status() == 303 || ctx.Response.Status() == 307
}

// IsForbidden returns boolean of this request is forbidden.
// HTTP 403 means forbidden.
func (ctx *Context) IsForbidden() bool {
	return ctx.Response.Status() == 403
}

// IsNotFound returns boolean of this request is not found.
// HTTP 404 means forbidden.
func (ctx *Context) IsNotFound() bool {
	return ctx.Response.Status() == 404
}

// IsClientError returns boolean of this request client sends error data.
// HTTP 4xx means forbidden.
func (ctx *Context) IsClientError() bool {
	return ctx.Response.Status() >= 400 && ctx.Response.Status() < 500
}

// IsServerError returns boolean of this server handler errors.
// HTTP 5xx means server internal error.
func (ctx *Context) IsServerError() bool {
	return ctx.Response.Status() >= 500 && ctx.Response.Status() < 600
}

// SetHeader sets response header item string via given key.
func (ctx *Context) SetHeader(key, val string) {
	ctx.Response.Header().Set(key, val)
}

// SetCookie sets cookie value via given key.
// others are ordered as cookie's max age time, path, domain, secure and httponly.
func (ctx *Context) SetCookie(name string, value string, others ...interface{}) {
	var cookie *http.Cookie = &http.Cookie{
		Name:  name,
		Value: value,
	}
	//fix cookie not work in IE
	if len(others) > 0 {
		var maxAge int
		switch v := others[0].(type) {
		case int:
			maxAge = v
		case int32:
			maxAge = int(v)
		case int64:
			maxAge = int(v)
		}
		switch {
		case maxAge > 0:
			cookie.Expires = time.Now().Add(time.Duration(maxAge) * time.Second)
			cookie.MaxAge = maxAge
		case maxAge < 0:
			cookie.MaxAge = 0
		}
	}
	// the settings below
	// Path, Domain, Secure, HttpOnly
	// can use nil skip set

	// default "/"
	if len(others) > 1 {
		if v, ok := others[1].(string); ok && len(v) > 0 {
			cookie.Path = v
		}
	} else {
		cookie.Path = "/"
	}

	// default empty
	if len(others) > 2 {
		if v, ok := others[2].(string); ok && len(v) > 0 {
			cookie.Domain = v
		}
	}

	// default empty
	if len(others) > 3 {
		var secure bool
		switch v := others[3].(type) {
		case bool:
			secure = v
		default:
			if others[3] != nil {
				secure = true
			}
		}
		cookie.Secure = secure
	}

	// default false. for session cookie default true
	httponly := false
	if len(others) > 4 {
		if v, ok := others[4].(bool); ok && v {
			// HttpOnly = true
			httponly = true
		}
	}
	cookie.HttpOnly = httponly

	ctx.Response.AddCookie(cookie)
}

// SetSecureCookie Set Secure cookie for response.
func (ctx *Context) SetSecureCookie(secret, name, value string, others ...interface{}) {
	vs := base64.URLEncoding.EncodeToString([]byte(value))
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	h := hmac.New(sha1.New, []byte(secret))
	fmt.Fprintf(h, "%s%s", vs, timestamp)
	sig := fmt.Sprintf("%02x", h.Sum(nil))
	cookie := strings.Join([]string{vs, timestamp, sig}, "|")
	ctx.SetCookie(name, cookie, others...)
}

// NoContent sends a response with no body and a status code.
func (ctx *Context) NoContent(status int) {
	ctx.Response.httpResponseWriter.WriteHeader(status)
}

// Bytes writes the data bytes to the connection as part of an HTTP reply.
func (ctx *Context) ResponseBytes(status int, contentType string, content []byte) error {
	if ctx.Response.Wrote() {
		ctx.Response.wroteCallback()
		return nil
	}
	ctx.Response.Header().Set(HeaderContentType, contentType)
	if ctx.enableGzip && len(ctx.Response.Header()[HeaderContentEncoding]) == 0 {
		buf := &bytes.Buffer{}
		ok, encoding, _ := acceptencoder.WriteBody(acceptencoder.ParseEncoding(ctx.Request), buf, content)
		if ok {
			ctx.Response.Header().Set(HeaderContentEncoding, encoding)
			content = buf.Bytes()
		}
	}
	ctx.Response.Header().Set(HeaderContentLength, strconv.Itoa(len(content)))
	_, err := ctx.Response.Write(content)
	return err
}

// String writes a string to the client, something like fmt.Fprintf
func (ctx *Context) ResponseString(status int, format string, s ...interface{}) error {
	if len(s) == 0 {
		return ctx.ResponseBytes(status, MIMETextPlainCharsetUTF8, []byte(format))
	}
	return ctx.ResponseBytes(status, MIMETextPlainCharsetUTF8, []byte(fmt.Sprintf(format, s...)))
}

// HTML sends an HTTP response with status code.
func (ctx *Context) ResponseHTML(status int, html string) error {
	x := (*[2]uintptr)(unsafe.Pointer(&html))
	h := [3]uintptr{x[0], x[1], x[1]}
	return ctx.ResponseBytes(status, MIMETextHTMLCharsetUTF8, *(*[]byte)(unsafe.Pointer(&h)))
}

// JSON sends a JSON response with status code.
func (ctx *Context) ResponseJSON(status int, data interface{}, isIndent ...bool) error {
	var (
		b   []byte
		err error
	)
	if len(isIndent) > 0 && isIndent[0] {
		b, err = json.MarshalIndent(data, "", "  ")
	} else {
		b, err = json.Marshal(data)
	}
	if err != nil {
		return err
	}
	return ctx.ResponseJSONBlob(status, b)
}

// JSONBlob sends a JSON blob response with status code.
func (ctx *Context) ResponseJSONBlob(status int, b []byte) error {
	return ctx.ResponseBytes(status, MIMEApplicationJSONCharsetUTF8, b)
}

// JSONP sends a JSONP response with status code. It uses `callback` to construct
// the JSONP payload.
func (ctx *Context) ResponseJSONP(status int, callback string, data interface{}, isIndent ...bool) error {
	var (
		b   []byte
		err error
	)
	if len(isIndent) > 0 && isIndent[0] {
		b, err = json.MarshalIndent(data, "", "  ")
	} else {
		b, err = json.Marshal(data)
	}
	if err != nil {
		return err
	}
	callback = template.JSEscapeString(callback)
	callbackContent := bytes.NewBufferString(" if(window." + callback + ")" + callback)
	callbackContent.WriteString("(")
	callbackContent.Write(b)
	callbackContent.WriteString(");\r\n")
	return ctx.ResponseBytes(status, MIMEApplicationJavaScriptCharsetUTF8, callbackContent.Bytes())
}

// XML sends an XML response with status code.
func (ctx *Context) ResponseXML(status int, data interface{}, isIndent ...bool) error {
	var (
		b   []byte
		err error
	)
	if len(isIndent) > 0 && isIndent[0] {
		b, err = xml.MarshalIndent(data, "", "  ")
	} else {
		b, err = xml.Marshal(data)
	}
	if err != nil {
		return err
	}
	return ctx.ResponseXMLBlob(status, b)
}

// XMLBlob sends a XML blob response with status code.
func (ctx *Context) ResponseXMLBlob(status int, b []byte) error {
	content := bytes.NewBufferString(xml.Header)
	content.Write(b)
	return ctx.ResponseBytes(status, MIMEApplicationXMLCharsetUTF8, content.Bytes())
}

// JSONOrXML serve Xml OR Json, depending on the value of the Accept header
func (ctx *Context) ResponseJSONOrXML(status int, data interface{}, isIndent ...bool) error {
	if ctx.AcceptJSON() || !ctx.AcceptXML() {
		return ctx.ResponseJSON(status, data, isIndent...)
	}
	return ctx.ResponseXML(status, data, isIndent...)
}

func (ctx *Context) ResponseDefault(code string, data interface{}, messages ...string) error {
	responseData := NewResponseData(code, data, messages...)
	err := ctx.Write(responseData)
	if err != nil {
		if ConnectionResetByPeer(err) || Timeout(err) || NetworkUnreachable(err) {
			return routing.NewHTTPError(499, "Response write error: "+err.Error())
		}
		return routing.NewHTTPError(http.StatusRequestTimeout, "Response write error: "+err.Error())
	}
	return err
}

// File forces response for download file.
// it prepares the download response header automatically.
func (ctx *Context) File(localFilename string, showFilename ...string) {
	ctx.Response.Header().Set(HeaderContentDescription, "File Transfer")
	ctx.Response.Header().Set(HeaderContentType, MIMEOctetStream)
	if len(showFilename) > 0 && showFilename[0] != "" {
		ctx.Response.Header().Set(HeaderContentDisposition, "attachment; filename="+showFilename[0])
	} else {
		ctx.Response.Header().Set(HeaderContentDisposition, "attachment; filename="+filepath.Base(localFilename))
	}
	ctx.Response.Header().Set(HeaderContentTransferEncoding, "binary")
	ctx.Response.Header().Set(HeaderExpires, "0")
	ctx.Response.Header().Set(HeaderCacheControl, "must-revalidate")
	ctx.Response.Header().Set(HeaderPragma, "public")
	http.ServeFile(ctx.ResponseWriter, ctx.Request, localFilename)
}
