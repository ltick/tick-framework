package api

import (
	"net/http"
	"reflect"

	"github.com/juju/errors"
	"github.com/ltick/tick-framework/utility"
)

var (
	errBindByName = "api: bind by name error"
)

type (
	// Handler is the main Faygo Handler interface.
	Handler interface {
		Serve(ctx *Context) error
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

// common errors
var (
	errNotStructPtr   = "api: handler must be a structure type or a structure pointer type"
	errNoParamHandler = "api: handler does not define any parameter tags"
)

// The default body decoder is json format decoding
var (
	defaultParamNameMapper = utility.SnakeString
)

// ToAPIHandler tries converts it to an *apiHandler.
func ToAPIHandler(handler Handler, noDefaultParams bool) (*apiHandler, error) {
	v := reflect.Indirect(reflect.ValueOf(handler))
	if v.Kind() != reflect.Struct {
		return nil, errors.New(errNotStructPtr)
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
		return nil, errors.New(errNoParamHandler)
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

// BindByName binds the net/http api params to a new struct and validate it.
func BindByName(
	apiName string,
	req *http.Request,
	apiParams ParamsKvstore,
) (
	interface{},
	error,
) {
	api, err := GetApi(apiName)
	if err != nil {
		return nil, errors.Annotate(err, errBindByName)
	}
	structPrinter, err := api.BindNew(req, apiParams)
	if err != nil {
		return nil, errors.Annotate(err, errBindByName)
	}
	return structPrinter, nil
}

// Bind binds the net/http api params to the `structPointer` param and validate it.
// note: structPointer must be struct pointe.
func Bind(
	structPointer interface{},
	req *http.Request,
	apiParams ParamsKvstore,
) error {
	api, err := GetApi(reflect.TypeOf(structPointer).String())
	if err != nil {
		return err
	}
	return api.BindAt(structPointer, req, apiParams)
}
