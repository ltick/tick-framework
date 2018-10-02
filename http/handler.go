package http

import (
	"errors"
	"reflect"
	"net/http"
	"fmt"
	"net/http/httputil"

	"github.com/ltick/tick-framework/utility"
	"github.com/ltick/tick-routing/proxy"
)

type (
	// Handler is the main Faygo Handler interface.
	Handler interface {
		Serve(ctx *Context) error
	}
	// APIHandler is the Faygo Handler interface,
	// which is implemented by a struct with API descriptor information.
	// It is an intelligent Handler of binding parameters.
	APIHandler interface {
		Handler
		APIDoc
	}
	// HandlerWithBody is the Faygo APIHandler interface but with DecodeBody method.
	HandlerWithBody interface {
		Handler
		BodyDecoder // Decode params from api body
	}
	// BodyDecoder is an interface to customize decoding operation
	BodyDecoder interface {
		Decode(dest interface{}, body []byte) error
	}
	// HandlerWithoutPath is handler without binding path parameter for middleware.
	HandlerWithoutPath interface {
		Handler
	}
	// APIDoc provides the API's note, result or parameters information.
	APIDoc interface {
		Doc() Doc
	}
	// APIParam is the api parameter information
	APIParam struct {
		Name     string      // Parameter name
		In       string      // The position of the parameter
		Required bool        // Is a required parameter
		Model    interface{} // A parameter value that is used to infer a value type and as a default value
		Desc     string      // Description
	}
	// Doc api information
	Doc struct {
		Note   string      `json:"note" xml:"note"`
		Return interface{} `json:"return,omitempty" xml:"return,omitempty"`
		// MoreParams extra added parameters definition
		MoreParams []APIParam `json:"more_params,omitempty" xml:"more_params,omitempty"`
	}
	// Notes implementation notes of a response
	Notes struct {
		Note   string      `json:"note" xml:"note"`
		Return interface{} `json:"return,omitempty" xml:"return,omitempty"`
	}
	// JSONMsg is commonly used to return JSON format response.
	JSONMsg struct {
		Code int         `json:"code" xml:"code"`                     // the status code of the business process (required)
		Info interface{} `json:"info,omitempty" xml:"info,omitempty"` // response's apiMap and example value (optional)
	}
	// apiHandler is an intelligent Handler of binding parameters.
	apiHandler struct {
		api *Api
	}
	// HandlerFunc type is an adapter to allow the use of
	// ordinary functions as HTTP handlers.  If f is a function
	// with the appropriate signature, HandlerFunc(f) is a
	// Handler that calls f.
	HandlerFunc func(ctx *Context) error
	// HandlerChain is the chain of handlers for a api.
	HandlerChain []Handler
	// ErrorFunc replies to the api with the specified error message and HTTP code.
	// It does not otherwise end the api; the caller should ensure no further
	// writes are done to ctx.
	// The error message should be plain text.
	ErrorFunc func(ctx *Context, errStr string, status int)
	// BinderrorFunc is called when binding or validation apiHandler parameters are wrong.
	BinderrorFunc func(ctx *Context, err error)
	// Bodydecoder decodes params from api body.
	Bodydecoder func(dest interface{}, body []byte) error
)

// Serve implements the Handler, is like ServeHTTP but for Faygo.
func (h HandlerFunc) Serve(ctx *Context) error {
	return h(ctx)
}

// common errors
var (
	ErrNotStructPtr   = errors.New("handler must be a structure type or a structure pointer type")
	ErrNoParamHandler = errors.New("handler does not define any parameter tags")
)
// The default body decoder is json format decoding
var (
	defaultParamNameMapper = utility.SnakeString
	defaultBinderrorFunc   = func(ctx *Context, err error) {
		ctx.Response.WriteHeader(http.StatusBadRequest)
		ctx.Response.Write(fmt.Sprintf("%v", err))
	}
)

var _ APIDoc = new(apiHandler)

// ToAPIHandler tries converts it to an *apiHandler.
func ToAPIHandler(handler Handler, noDefaultParams bool) (*apiHandler, error) {
	v := reflect.Indirect(reflect.ValueOf(handler))
	if v.Kind() != reflect.Struct {
		return nil, ErrNotStructPtr
	}

	var structPointer = v.Addr().Interface()
	var bodydecoder = defaultBodydecoder
	if h, ok := structPointer.(HandlerWithBody); ok {
		bodydecoder = h.Decode
	}

	api, err := NewApi(structPointer, defaultParamNameMapper, bodydecoder, !noDefaultParams)
	if err != nil {
		return nil, err
	}
	if api.Number() == 0 {
		return nil, ErrNoParamHandler
	}

	// Reduce the creation of unnecessary field paramValues.
	return &apiHandler{
		api: api,
	}, nil
}

// IsHandlerWithoutPath verifies that the Handler is an HandlerWithoutPath.
func IsHandlerWithoutPath(handler Handler, noDefaultParams bool) bool {
	v := reflect.Indirect(reflect.ValueOf(handler))
	if v.Kind() != reflect.Struct {
		return true
	}
	api, err := NewApi(v.Addr().Interface(), nil, nil, !noDefaultParams)
	if err != nil {
		return true
	}
	for _, param := range api.Params() {
		if param.In() == "path" {
			return false
		}
	}
	return true
}

// Serve implements the APIHandler.
// creates a new `*apiHandler`;
// binds the api path params to `apiHandler.handler`;
// calls Handler.Serve() method.
func (h *apiHandler) Serve(ctx *Context) error {
	obj, err := h.api.BindNew(ctx.Request, ctx.apiParams)
	if err != nil {
		defaultBinderrorFunc(ctx, err)
		ctx.Abort()
		return nil
	}
	return obj.(Handler).Serve(ctx)
}

// Doc returns the API's note, result or parameters information.
func (h *apiHandler) Doc() Doc {
	var doc Doc
	if d, ok := h.api.Raw().(APIDoc); ok {
		doc = d.Doc()
	}
	for _, param := range h.api.Params() {
		var had bool
		var info = APIParam{
			Name:     param.Name(),
			In:       param.In(),
			Required: param.IsRequired(),
			Desc:     param.Description(),
			Model:    param.Raw(),
		}
		for i, p := range doc.MoreParams {
			if p.Name == info.Name {
				doc.MoreParams[i] = info
				had = true
				break
			}
		}
		if !had {
			doc.MoreParams = append(doc.MoreParams, info)
		}
	}
	return doc
}
