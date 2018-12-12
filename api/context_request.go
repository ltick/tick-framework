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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"github.com/ltick/tick-framework/utility"
)

// Regexes for checking the accept headers
// TODO make sure these are correct
var (
	acceptsHTMLRegex = regexp.MustCompile(`(text/html|application/xhtml\+xml)(?:,|$)`)
	acceptsXMLRegex  = regexp.MustCompile(`(application/xml|text/xml)(?:,|$)`)
	acceptsJSONRegex = regexp.MustCompile(`(application/json)(?:,|$)`)
)

// Protocol returns request protocol name, such as HTTP/1.1 .
func (ctx *Context) Protocol() string {
	return ctx.Request.Proto
}

// URI returns full request url with query string, fragment.
func (ctx *Context) URI() string {
	return ctx.Request.RequestURI
}

// URL returns full request url with query string, fragment.
func (ctx *Context) URL() *url.URL {
	return ctx.Request.URL
}

// Path returns request url path (without query string, fragment).
func (ctx *Context) Path() string {
	return ctx.Request.URL.Path
}

// ModifyPath modifies the access path for the request.
func (ctx *Context) ModifyPath(p string) {
	ctx.Request.URL.Path = p
}

// Scheme returns request scheme as "http" or "https".
func (ctx *Context) Scheme() string {
	if scheme := ctx.HeaderParam(HeaderXForwardedProto); scheme != "" {
		return scheme
	}
	if ctx.Request.URL.Scheme != "" {
		return ctx.Request.URL.Scheme
	}
	if ctx.Request.TLS == nil {
		return "http"
	}
	return "https"
}

// Site returns base site url as `scheme://domain:port` type.
func (ctx *Context) Site() string {
	return ctx.Scheme() + "://" + ctx.Request.Host
}

// Host returns a host:port string for this request,
// such as "www.example.com" or "www.example.com:8080".
func (ctx *Context) Host() string {
	return ctx.Request.Host
}

// Domain returns domain as `www.example.com` style.
func (ctx *Context) Domain() string {
	hostport := ctx.Request.Host
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return hostport
	}
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	return hostport[:colon]
}

// Port returns the port number of request.
func (ctx *Context) Port() int {
	portStr := portString(ctx.Request.Host)
	if len(portStr) == 0 {
		return 80
	}
	port, _ := strconv.Atoi(portStr)
	return port
}

func portString(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return ""
	}
	if i := strings.Index(hostport, "]:"); i != -1 {
		return hostport[i+len("]:"):]
	}
	if strings.Contains(hostport, "]") {
		return ""
	}
	return hostport[colon+len(":"):]
}

// IP gets just the ip from the most direct one client.
func (ctx *Context) IP() string {
	ip := strings.Split(ctx.Request.RemoteAddr, ":")[0]
	if len(ip) == 0 {
		return ""
	}
	if ip[0] != '[' {
		return ip
	}
	return "127.0.0.1"
}

// RealIP returns request client ip.
// if in proxy, return first proxy id.
// if error, return 127.0.0.1.
func (ctx *Context) RealIP() string {
	var ip = ctx.Request.Header.Get(HeaderXRealIP)
	if len(ip) > 0 {
		return ip
	}
	ips := ctx.Proxy()
	if len(ips) > 0 && ips[0] != "" {
		ip = strings.Split(ips[0], ":")[0]
		if len(ip) == 0 {
			return ""
		}
		if ip[0] != '[' {
			return ip
		}
		return "127.0.0.1"
	}
	return ctx.IP()
}

// Proxy returns proxy client ips slice.
func (ctx *Context) Proxy() []string {
	if ips := ctx.HeaderParam(HeaderXForwardedFor); ips != "" {
		return strings.Split(ips, ",")
	}
	return []string{}
}

// Referer returns http referer header.
func (ctx *Context) Referer() string {
	return ctx.HeaderParam(HeaderReferer)
}

// Method returns http request method.
func (ctx *Context) Method() string {
	return ctx.Request.Method
}

// Is returns boolean of this request is on given method, such as Is("POST").
func (ctx *Context) Is(method string) bool {
	return ctx.Method() == method
}

// IsGet Is this a GET method request?
func (ctx *Context) IsGet() bool {
	return ctx.Is("GET")
}

// IsPost Is this a POST method request?
func (ctx *Context) IsPost() bool {
	return ctx.Is("POST")
}

// IsHead Is this a Head method request?
func (ctx *Context) IsHead() bool {
	return ctx.Is("HEAD")
}

// IsOptions Is this a OPTIONS method request?
func (ctx *Context) IsOptions() bool {
	return ctx.Is("OPTIONS")
}

// IsPut Is this a PUT method request?
func (ctx *Context) IsPut() bool {
	return ctx.Is("PUT")
}

// IsDelete Is this a DELETE method request?
func (ctx *Context) IsDelete() bool {
	return ctx.Is("DELETE")
}

// IsPatch Is this a PATCH method request?
func (ctx *Context) IsPatch() bool {
	return ctx.Is("PATCH")
}

// IsAjax returns boolean of this request is generated by ajax.
func (ctx *Context) IsAjax() bool {
	return ctx.HeaderParam(HeaderXRequestedWith) == "XMLHttpRequest"
}

// IsSecure returns boolean of this request is in https.
func (ctx *Context) IsSecure() bool {
	return ctx.Scheme() == "https"
}

// IsWebsocket returns boolean of this request is in webSocket.
func (ctx *Context) IsWebsocket() bool {
	return ctx.HeaderParam(HeaderUpgrade) == "websocket"
}

// IsUpload returns boolean of whether file uploads in this request or not..
func (ctx *Context) IsUpload() bool {
	return strings.Contains(ctx.HeaderParam(HeaderContentType), MIMEMultipartForm)
}

// AcceptHTML Checks if request accepts html response
func (ctx *Context) AcceptHTML() bool {
	return acceptsHTMLRegex.MatchString(ctx.HeaderParam(HeaderAccept))
}

// AcceptXML Checks if request accepts xml response
func (ctx *Context) AcceptXML() bool {
	return acceptsXMLRegex.MatchString(ctx.HeaderParam(HeaderAccept))
}

// AcceptJSON Checks if request accepts json response
func (ctx *Context) AcceptJSON() bool {
	return acceptsJSONRegex.MatchString(ctx.HeaderParam(HeaderAccept))
}

// UserAgent returns request client user agent string.
func (ctx *Context) UserAgent() string {
	return ctx.HeaderParam(HeaderUserAgent)
}

// Param returns the first value for the kinds of parameters.
// priority:
// path parameters > POST and PUT body parameters > URL query string values > header > cookie.Value.
//
// Param calls ParseMultipartForm and ParseForm if necessary and ignores
// any errors returned by these functions.
// If key is not present, Param returns the empty string.
// To access multiple values of the same key, call ParseForm and
// then inspect Request.Form directly.
func (ctx *Context) Param(key string) string {
	var value string
	value = ctx.Context.Param(key)
	if len(value) > 0 {
		return value
	}
	value = ctx.Request.Header.Get(key)
	if len(value) > 0 {
		return value
	}
	if cookie, _ := ctx.Request.Cookie(key); cookie != nil {
		return cookie.Value
	}
	return value
}

// ParseFormOrMulitForm parseForm or parseMultiForm based on Content-type
func (ctx *Context) ParseFormOrMulitForm(maxMemory int64) error {
	return ctx.Request.ParseMultipartForm(maxMemory)
}

// FormParam returns the first value for the named component of the POST or PUT ruest body.
// URL query parameters and path parameters are ignored.
// FormParam calls ParseMultipartForm and ParseForm if necessary and ignores
// any errors returned by these functions.
// If key is not present, FormParam returns the empty string.
func (ctx *Context) FormParam(key string) string {
	ctx.makeSureParseMultipartForm()
	return ctx.Request.PostFormValue(key)
}

// FormParams returns the form field value with "[]string" for the provided key.
func (ctx *Context) FormParams(key string) []string {
	ctx.makeSureParseMultipartForm()
	return ctx.Request.PostForm[key]
}

// FormParamAll returns the parsed form data from POST, PATCH,
// or PUT body parameters.
func (ctx *Context) FormParamAll() url.Values {
	ctx.makeSureParseMultipartForm()
	return ctx.Request.PostForm
}

// BindForm reads form data from request's body
func (ctx *Context) BindForm(structObject interface{}) error {
	value := reflect.ValueOf(structObject)
	if value.Kind() != reflect.Ptr {
		return errors.New("`*Context.BindForm` accepts only parameter of struct pointer type")
	}
	value = reflect.Indirect(value)
	if value.Kind() != reflect.Struct {
		return errors.New("`*Context.BindForm` accepts only parameter of struct pointer type")
	}
	t := value.Type()
	for i, count := 0, t.NumField(); i < count; i++ {
		fieldT := t.Field(i)
		if fieldT.Anonymous {
			continue
		}
		var key = fieldT.Tag.Get(TAG_PARAM)
		if key == "" {
			key = ctx.paramNameMapper(fieldT.Name)
		}
		err := utility.ConvertAssign(value.Field(i), ctx.FormParams(key)...)
		if err != nil {
			return err
		}
	}
	return nil
}

// HeaderParam gets the first header value associated with the given key.
// If there are no values associated with the key, HeaderParam returns
// the empty string.
func (ctx *Context) HeaderParam(key string) string {
	return ctx.Request.Header.Get(key)
}

// HeaderParamAll returns the whole ruest header.
func (ctx *Context) HeaderParamAll() http.Header {
	return ctx.Request.Header
}

// CookieParam returns request cookie item string by a given key.
// if non-existed, return empty string.
func (ctx *Context) CookieParam(key string) string {
	cookie, err := ctx.Request.Cookie(key)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// SecureCookieParam Get secure cookie from request by a given key.
func (ctx *Context) SecureCookieParam(secret, key string) (string, bool) {
	val := ctx.CookieParam(key)
	if val == "" {
		return "", false
	}

	parts := strings.SplitN(val, "|", 3)

	if len(parts) != 3 {
		return "", false
	}

	vs := parts[0]
	timestamp := parts[1]
	sig := parts[2]

	h := hmac.New(sha1.New, []byte(secret))
	fmt.Fprintf(h, "%s%s", vs, timestamp)

	if fmt.Sprintf("%02x", h.Sum(nil)) != sig {
		return "", false
	}
	res, _ := base64.URLEncoding.DecodeString(vs)
	return utility.BytesToString(res), true
}

// FormFile returns the first file for the provided form key.
// FormFile calls ParseMultipartForm and ParseForm if necessary.
func (ctx *Context) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	ctx.makeSureParseMultipartForm()
	return ctx.Request.FormFile(key)
}

func (ctx *Context) makeSureParseMultipartForm() {
	if ctx.Request.PostForm == nil || ctx.Request.MultipartForm == nil {
		ctx.Request.ParseMultipartForm(ctx.multipartMaxMemory)
	}
}

// HasFormFile returns if the file header for the provided form key is exist.
func (ctx *Context) HasFormFile(key string) bool {
	ctx.makeSureParseMultipartForm()
	if ctx.Request.MultipartForm != nil && ctx.Request.MultipartForm.File != nil {
		if fhs := ctx.Request.MultipartForm.File[key]; len(fhs) > 0 {
			return true
		}
	}
	return false
}

// StoredFileInfo for StoreFiles()
type StoredFileInfo struct {
	Url  string
	Size int64
}

// StoreFile saves the uploaded file to global.ctx.fileStoreDir,
// character "?" indicates that the original file name.
// for example overwriteFilenames="a/?" -> global.ctx.fileStoreDir/a/fname.
func (ctx *Context) StoreFile(key string, cover bool, overwriteFilenames ...string) (storedFileInfo StoredFileInfo, err error) {
	f, fh, err := ctx.Request.FormFile(key)
	if err != nil {
		return
	}
	defer func() {
		err2 := f.Close()
		if err2 != nil && err == nil {
			err = err2
		}
	}()

	ctx.fixFilename(fh)
	var filename = filepath.Base(fh.Filename)

	// Sets the full file name
	var fullname string
	if len(overwriteFilenames) == 0 {
		fullname = filepath.Join(ctx.fileStoreDir, filename)
	} else {
		if strings.Contains(overwriteFilenames[0], "?") {
			fullname = filepath.Join(ctx.fileStoreDir, strings.Replace(overwriteFilenames[0], "?", filename, -1))
		} else {
			fname := strings.TrimRight(overwriteFilenames[0], ".")
			if filepath.Ext(fname) == "" {
				fullname = filepath.Join(ctx.fileStoreDir, fname+filepath.Ext(filename))
			} else {
				fullname = filepath.Join(ctx.fileStoreDir, fname)
			}
		}
	}

	// Create the completion file path
	p, _ := filepath.Split(fullname)
	err = os.MkdirAll(p, 0777)
	if err != nil {
		return
	}

	// If the file with the same name exists, add the suffix of the serial number
	idx := strings.LastIndex(fullname, filepath.Ext(fullname))
	_fullname := fullname
	for i := 2; utility.FileExists(_fullname) && !cover; i++ {
		_fullname = fmt.Sprintf("%s(%d)%s", fullname[:idx], i, fullname[idx:])
	}
	fullname = _fullname

	// Create the URL of the file
	storedFileInfo.Url = "/" + strings.Replace(fullname, `\`, `/`, -1)

	// Save the file to local
	f2, err := os.OpenFile(fullname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	storedFileInfo.Size, err = io.Copy(f2, f)
	err3 := f2.Close()
	if err3 != nil && err == nil {
		err = err3
	}
	return
}

// StoreFiles saves the uploaded files to global.ctx.fileStoreDir,
// it's similar to StoreFile, but for saving multiple files.
func (ctx *Context) StoreFiles(key string, cover bool, overwriteFilenames ...string) (storedFileInfos []StoredFileInfo, err error) {
	if !ctx.HasFormFile(key) {
		err = errors.New("there are no file param: " + key)
		return
	}
	files := ctx.Request.MultipartForm.File[key]
	hasFilename := len(overwriteFilenames) > 0
	filemap := map[string]int{}
	for _, fh := range files {
		var f multipart.File
		f, err = fh.Open()
		if err != nil {
			return
		}
		defer func() {
			err2 := f.Close()
			if err2 != nil && err == nil {
				err = err2
			}
		}()

		ctx.fixFilename(fh)
		var filename = filepath.Base(fh.Filename)

		// Sets the full file name
		var fullname string
		if !hasFilename {
			fullname = filepath.Join(ctx.fileStoreDir, filename)
		} else {
			if strings.Contains(overwriteFilenames[0], "?") {
				fullname = filepath.Join(ctx.fileStoreDir, strings.Replace(overwriteFilenames[0], "?", filename, -1))
			} else {
				fname := strings.TrimRight(overwriteFilenames[0], ".")
				if filepath.Ext(fname) == "" {
					fullname = filepath.Join(ctx.fileStoreDir, fname+filepath.Ext(filename))
				} else {
					fullname = filepath.Join(ctx.fileStoreDir, fname)
				}
			}
		}

		// If the file with the same name exists, add the suffix of the serial number
		idx := strings.LastIndex(fullname, filepath.Ext(fullname))
		num := filemap[fullname]
		_fullname := fullname
		num++
		if num >= 2 {
			_fullname = fmt.Sprintf("%s(%d)%s", fullname[:idx], num, fullname[idx:])
		}
		for utility.FileExists(_fullname) && !cover {
			num++
			_fullname = fmt.Sprintf("%s(%d)%s", fullname[:idx], num, fullname[idx:])
		}
		filemap[fullname] = num
		fullname = _fullname

		var info StoredFileInfo

		// Create the URL of the file
		info.Url = "/" + strings.Replace(fullname, `\`, `/`, -1)

		// Save the file to local
		var f2 *os.File
		f2, err = os.OpenFile(fullname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			// Create the completion file path
			p, _ := filepath.Split(fullname)
			err = os.MkdirAll(p, 0777)
			if err != nil {
				return
			}
			f2, err = os.OpenFile(fullname, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return
			}
		}
		info.Size, err = io.Copy(f2, f)
		err3 := f2.Close()
		if err3 != nil && err == nil {
			err = err3
			return
		}
		storedFileInfos = append(storedFileInfos, info)
	}
	return
}

func (ctx *Context) fixFilename(fh *multipart.FileHeader) {
	if strings.Contains(fh.Filename, ":") {
		sub := `"; filename="`
		disp := fh.Header.Get("Content-Disposition")
		idx := strings.Index(disp, sub)
		if idx != -1 {
			sub = disp[idx+len(sub):]
			idx = strings.Index(sub, `"`)
			if idx != -1 {
				fh.Filename = sub[:idx]
			}
		}
	}
	fh.Filename = strings.TrimRight(fh.Filename, "/")
	fh.Filename = strings.TrimRight(fh.Filename, "\\")
	m := strings.LastIndex(fh.Filename, "/")
	n := strings.LastIndex(fh.Filename, "\\")
	if m > n {
		fh.Filename = fh.Filename[m+1:]
	} else if n > m {
		fh.Filename = fh.Filename[n+1:]
	}
}

// BindJSON reads JSON from request's body
func (ctx *Context) BindJSON(jsonObject interface{}) error {
	rawData, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		return err
	}

	// check if jsonObject is already a pointer, if yes then pass as it's
	if reflect.TypeOf(jsonObject).Kind() == reflect.Ptr {
		err := json.Unmarshal(rawData, jsonObject)
		if err != nil {
			return err
		}
	}
	// finally, if the jsonObject is not a pointer
	return json.Unmarshal(rawData, &jsonObject)
}

// BindXML reads XML from request's body
func (ctx *Context) BindXML(xmlObject interface{}) error {
	rawData, _ := ioutil.ReadAll(ctx.Request.Body)
	// check if xmlObject is already a pointer, if yes then pass as it's
	if reflect.TypeOf(xmlObject).Kind() == reflect.Ptr {
		err := xml.Unmarshal(rawData, xmlObject)
		if err != nil {
			return err
		}
	}
	// finally, if the xmlObject is not a pointer
	return xml.Unmarshal(rawData, &xmlObject)
}

// LimitedBodyBytes returns the raw request body data as bytes.
// Note:
//  1.limited by maximum length;
//  2.if frame.config.PrintBody==false and ctx.Request.Body is readed, returns nil;
//  3.if ctx.IsUpload()==true and ctx.Request.Body is readed, returns nil.
func (ctx *Context) LimitedBodyBytes() []byte {
	if ctx.memoryStoreByte != nil {
		return ctx.memoryStoreByte
	}
	if ctx.Request.Body == nil {
		ctx.memoryStoreByte = []byte{}
		return ctx.memoryStoreByte
	}
	safe := &io.LimitedReader{R: ctx.Request.Body, N: ctx.multipartMaxMemory}
	buf := bytes.NewBuffer(make([]byte, 0, bytes.MinRead))
	buf.ReadFrom(safe)
	ctx.memoryStoreByte = buf.Bytes()
	ctx.Request.Body = ioutil.NopCloser(io.MultiReader(buf, ctx.Request.Body))
	return ctx.memoryStoreByte
}