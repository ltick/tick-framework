package http

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"sync"

	"github.com/juju/errors"
	"github.com/ltick/ltick-validation"
	"github.com/ltick/tick-framework/utility"
)

type (
	// Api defines a parameter model for an web api.
	Api struct {
		name   string
		params []*Param
		//used to create a new struct (non-pointer)
		structType reflect.Type
		//the raw struct pointer
		rawStructPointer interface{}
		// rawStructPointer value bytes
		defaultValues []byte
		// create param name from struct field name
		paramNameNormalizer ParamNameMapper
		// decode params from request body
		bodydecoder Bodydecoder
		//when request Content-Type is multipart/form-data, the max memory for body.
		maxMemory int64
	}
	// ApiMap is a collection of Api
	ApiMap struct {
		Map map[string]*Api
		sync.RWMutex
	}

	ParamsKVStore interface {
		Get(k string) (v string, found bool)
	}
	// ApiParam is a single URL parameter, consisting of a key and a value.
	ApiParam struct {
		Key   string
		Value string
	}
	// ApiParams is a Param-slice, as returned by the routea.
	// The slice is ordered, the first URL parameter is also the first slice value.
	// It is therefore safe to read values by the index.
	ApiParams []ApiParam
	// Map is just a conversion for a map[string]string
	Map map[string]string
)

func (m Map) Get(k string) (string, bool) {
	v, found := m[k]
	return v, found
}

var _ ParamsKVStore = ApiParams{}

// ByName returns the value of the first ApiParam which key matches the given name.
// If no matching ApiParam is found, an empty string is returned.
func (ps ApiParams) ByName(name string) string {
	for i := range ps {
		if ps[i].Key == name {
			return ps[i].Value
		}
	}
	return ""
}

// Get returns the value of the first ApiParam which key matches the given name.
// It implements the ParamsKVStore interface.
func (ps ApiParams) Get(name string) (string, bool) {
	for i := range ps {
		if ps[i].Key == name {
			return ps[i].Value, true
		}
	}
	return "", false
}

// Replace changes the value of the first ApiParam which key matches the given name.
// If n < 0, there is no limit on the number of changed.
// If the key is changed, return true.
func (ps ApiParams) Replace(name string, value string, n int) bool {
	if n < 0 {
		n = len(ps)
	}
	var changed bool
	for i := range ps {
		if n <= 0 {
			break
		}
		if ps[i].Key == name {
			ps[i].Value = value
			changed = true
			n--
		}
	}
	return changed
}

// The default body decoder is json format decoding
var (
	defaultBodydecoder = func(dest interface{}, body []byte) (err error) {
		return json.Unmarshal(body, dest)
	}
)
var (
	defaultApiMap = &ApiMap{
		Map: map[string]*Api{},
	}
)

func GetApi(ApiName string) (*Api, error) {
	api, ok := defaultApiMap.get(ApiName)
	if !ok {
		return nil, errors.New("struct `" + ApiName + "` is not registered")
	}
	return api, nil
}

// SetApi caches `*Api`
func SetApi(api *Api) {
	defaultApiMap.set(api)
}

// NewApi parses and store the struct object, requires a struct pointer,
// if `paramNameNormalizer` is nil, `paramNameNormalizer=toSnake`,
// if `bodydecoder` is nil, `bodydecoder=bodyJSON`,
func NewApi(
	structPointer interface{},
	paramNameNormalizer ParamNameMapper,
	bodydecoder Bodydecoder,
	useDefaultValues bool,
) (
	*Api,
	error,
) {
	name := reflect.TypeOf(structPointer).String()
	v := reflect.ValueOf(structPointer)
	if v.Kind() != reflect.Ptr {
		return nil, errors.Trace(fmt.Errorf("%s|%s|%s", name, "*", "the binding object must be a struct pointer"))
	}
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return nil, errors.Trace(fmt.Errorf("%s|%s|%s", name, "*", "the binding object must be a struct pointer"))
	}
	var api = &Api{
		name:             name,
		params:           []*Param{},
		structType:       v.Type(),
		rawStructPointer: structPointer,
	}
	if paramNameNormalizer != nil {
		api.paramNameNormalizer = paramNameNormalizer
	} else {
		api.paramNameNormalizer = utility.SnakeString
	}
	if bodydecoder != nil {
		api.bodydecoder = bodydecoder
	} else {
		api.bodydecoder = defaultBodydecoder
	}
	err := api.addFields([]int{}, api.structType, v)
	if err != nil {
		return nil, err
	}

	if useDefaultValues && !reflect.DeepEqual(reflect.New(api.structType).Interface(), api.rawStructPointer) {
		buf := bytes.NewBuffer(nil)
		err = gob.NewEncoder(buf).EncodeValue(v)
		if err == nil {
			api.defaultValues = buf.Bytes()
		}
	}
	defaultApiMap.set(api)
	return api, nil
}

// Raw returns the Api's original value
func (a *Api) Raw() interface{} {
	return a.rawStructPointer
}
func (a *Api) addFields(parentIndexPath []int, t reflect.Type, v reflect.Value) error {
	var err error
	var maxMemoryMB int64
	var hasFormData, hasBody bool
	var deep = len(parentIndexPath) + 1
	for i := 0; i < t.NumField(); i++ {
		indexPath := make([]int, deep)
		copy(indexPath, parentIndexPath)
		indexPath[deep-1] = i

		var field = t.Field(i)
		tag, ok := field.Tag.Lookup(TAG_PARAM)
		if !ok {
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				if err = a.addFields(indexPath, field.Type, v.Field(i)); err != nil {
					return err
				}
			}
			continue
		}

		if tag == TAG_IGNORE_PARAM {
			continue
		}
		if field.Type.Kind() == reflect.Ptr && field.Type.String() != fileTypeString && field.Type.String() != cookieTypeString {
			return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "field can not be a pointer"))
		}

		var value = v.Field(i)
		if !value.CanSet() {
			return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "field can not be a unexported field"))
		}

		var parsedTags = ParseTags(tag)
		var paramPosition = parsedTags[KEY_IN]
		var paramTypeString = field.Type.String()

		switch paramTypeString {
		case fileTypeString, filesTypeString, fileTypeString2, filesTypeString2:
			if paramPosition != "formData" {
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "when field type is `"+paramTypeString+"`, tag `in` value must be `formData`"))
			}
		case cookieTypeString, cookieTypeString2 /*, fasthttpCookieTypeString*/ :
			if paramPosition != "cookie" {
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "when field type is `"+paramTypeString+"`, tag `in` value must be `cookie`"))
			}
		}

		switch paramPosition {
		case "formData":
			if hasBody {
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "tags of `in(formData)` and `in(body)` can not exist at the same time"))
			}
			hasFormData = true
		case "body":
			if hasFormData {
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "tags of `in(formData)` and `in(body)` can not exist at the same time"))
			}
			if hasBody {
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "there should not be more than one tag `in(body)`"))
			}
			hasBody = true
		case "path":
			parsedTags[KEY_REQUIRED] = KEY_REQUIRED
			// case "cookie":
			// 	switch paramTypeString {
			// 	case cookieTypeString, fasthttpCookieTypeString, stringTypeString, bytesTypeString, bytes2TypeString:
			// 	default:
			// 		return NewError( t.String(),field.Name, "invalid field type for `in(cookie)`, refer to the following: `http.Cookie`, `fasthttp.Cookie`, `string`, `[]byte` or `[]uint8`")
			// 	}
		default:
			if !TagInValues[paramPosition] {
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "invalid tag `in` value, refer to the following: `path`, `query`, `formData`, `body`, `header` or `cookie`"))
			}
		}
		if _, ok := parsedTags[KEY_LEN]; ok {
			switch paramTypeString {
			case "string", "[]string", "[]int", "[]int8", "[]int16", "[]int32", "[]int64", "[]uint", "[]uint8", "[]uint16", "[]uint32", "[]uint64", "[]float32", "[]float64":
			default:
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "invalid `len` tag for non-string or non-basetype-slice field"))
			}
		}
		if _, ok := parsedTags[KEY_RANGE]; ok {
			switch paramTypeString {
			case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			case "[]int", "[]int8", "[]int16", "[]int32", "[]int64", "[]uint", "[]uint8", "[]uint16", "[]uint32", "[]uint64", "[]float32", "[]float64":
			default:
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "invalid `range` tag for non-number field"))
			}
		}
		if _, ok := parsedTags[KEY_REGEXP]; ok {
			if paramTypeString != "string" && paramTypeString != "[]string" {
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "invalid `"+KEY_REGEXP+"` tag for non-string field"))
			}
		}
		if a, ok := parsedTags[KEY_MAXMB]; ok {
			i, err := strconv.ParseInt(a, 10, 64)
			if err != nil {
				return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "invalid `maxmb` tag, it must be positive integer"))
			}
			if i > maxMemoryMB {
				maxMemoryMB = i
			}
		}

		fd := &Param{
			apiName:   a.name,
			indexPath: indexPath,
			tags:      parsedTags,
			rawTag:    field.Tag,
			rawValue:  value,
		}

		if errStr, ok := fd.tags[KEY_ERR]; ok {
			fd.err = errors.New(errStr)
		}

		// fmt.Printf("%#v\n", fd.tags)

		if fd.name, ok = parsedTags[KEY_NAME]; !ok {
			fd.name = a.paramNameNormalizer(field.Name)
		}
		if paramPosition == "header" {
			fd.name = textproto.CanonicalMIMEHeaderKey(fd.name)
		}

		fd.isFile = paramTypeString == fileTypeString || paramTypeString == filesTypeString || paramTypeString == fileTypeString2 || paramTypeString == filesTypeString2

		_, fd.isRequired = parsedTags[KEY_REQUIRED]
		_, hasNonzero := parsedTags[KEY_NOTEMPTY]
		if !fd.isRequired && (hasNonzero || len(parsedTags[KEY_RANGE]) > 0) {
			fd.isRequired = true
		}

		if err = fd.makeVerifyRules(); err != nil {
			return errors.Trace(fmt.Errorf("%s|%s|%s", t.String(), field.Name, "initial validation failed:"+err.Error()))
		}

		a.params = append(a.params, fd)
	}
	if maxMemoryMB > 0 {
		a.maxMemory = maxMemoryMB * MB
	} else {
		a.maxMemory = defaultMaxMemory
	}
	return nil
}

// Number returns the number of parameters to be bound
func (a *Api) Number() int {
	return len(a.params)
}

// BindAt binds the net/http api params to a struct pointer and validate it.
// note: structPointer must be struct pointea.
func (a *Api) BindAt(
	structPointer interface{},
	req *http.Request,
	apiParams ParamsKVStore,
) error {
	name := reflect.TypeOf(structPointer).String()
	if name != a.name {
		return errors.New("the structPointer's type `" + name + "` does not match type `" + a.name + "`")
	}
	return a.BindFields(
		a.fieldsForBinding(reflect.ValueOf(structPointer).Elem()),
		req,
		apiParams,
	)
}

// BindNew binds the net/http api params to a struct pointer and validate it.
func (a *Api) BindNew(
	req *http.Request,
	apiParams ParamsKVStore,
) (
	interface{},
	error,
) {
	structPrinter, fields := a.NewReceiver()
	err := a.BindFields(fields, req, apiParams)
	return structPrinter, err
}

// NewReceiver creates a new struct pointer and the field's values  for its receive parameterste it.
func (a *Api) NewReceiver() (interface{}, []interface{}) {
	object := reflect.New(a.structType)
	if len(a.defaultValues) > 0 {
		// fmt.Printf("setting default value: %s\n", a.structType.String())
		de := gob.NewDecoder(bytes.NewReader(a.defaultValues))
		err := de.DecodeValue(object.Elem())
		if err != nil {
			panic(err)
		}
	}
	return object.Interface(), a.fieldsForBinding(object.Elem())
}

func (a *Api) fieldsForBinding(structElem reflect.Value) []interface{} {
	count := len(a.params)
	fields := make([]interface{}, count)
	for i := 0; i < count; i++ {
		value := structElem
		param := a.params[i]
		for _, index := range param.indexPath {
			value = value.Field(index)
		}
		fields[i] = value
	}
	return fields
}

// BindFields binds the net/http api params to a struct and validate it.
// Must ensure that the param `fields` matches `a.params`.
func (a *Api) BindFields(
	fields []interface{},
	req *http.Request,
	apiParams ParamsKVStore,
) (
	err error,
) {
	if apiParams == nil {
		apiParams = Map(map[string]string{})
	}
	if req.Form == nil {
		req.ParseMultipartForm(a.maxMemory)
	}
	var queryValues url.Values
	defer func() {
		if p := recover(); p != nil {
			err = errors.Trace(fmt.Errorf("%s|%s|%s", a.name, "?", fmt.Sprint(p)))
		}
	}()
	var ok bool
	for i, param := range a.params {
		value := fields[i]
		switch param.In() {
		case "path":
			value, ok = apiParams.Get(param.name)
			if !ok {
				return param.Error("missing path param")
			}
			// fmt.Printf("paramName:%s\nvalue:%#v\n\n", param.name, paramValue)
		case "query":
			if queryValues == nil {
				queryValues, err = url.ParseQuery(req.URL.RawQuery)
				if err != nil {
					queryValues = make(url.Values)
				}
			}
			value, ok = queryValues[param.name]
			if !ok && param.IsRequired() {
				return param.Error("missing query param")
			}

		case "formData":
			// Can not exist with `body` param at the same time
			if param.IsFile() {
				if req.MultipartForm != nil {
					fhs := req.MultipartForm.File[param.name]
					if len(fhs) == 0 {
						if param.IsRequired() {
							return param.Error("missing formData param")
						}
						continue
					}
					typ := reflect.TypeOf(value)
					switch typ.String() {
					case fileTypeString:
						value = fhs[0]
					case fileTypeString2:
						value = &fhs[0]
					case filesTypeString:
						value = fhs
					case filesTypeString2:
						fhs2 := make([]multipart.FileHeader, len(fhs))
						for i, fh := range fhs {
							fhs2[i] = *fh
						}
						value = fhs2
					default:
						return param.Error(
							"the param type is incorrect, reference: " +
								fileTypeString +
								"," + filesTypeString,
						)
					}
				} else if param.IsRequired() {
					return param.Error("missing formData param")
				}
				continue
			}

			value, ok = req.PostForm[param.name]
			if !ok && param.IsRequired() {
				return param.Error("missing formData param")
			}

		case "body":
			// Theoretically there should be at most one `body` param, and can not exist with `formData` at the same time
			var body []byte
			body, err = ioutil.ReadAll(req.Body)
			req.Body.Close()
			if err == nil {
				if err = a.bodydecoder(value, body); err != nil {
					return param.Error(err.Error())
				}
			} else if param.IsRequired() {
				return param.Error("missing body param")
			}

		case "header":
			value, ok = req.Header[param.name]
			if !ok && param.IsRequired() {
				return param.Error("missing header param")
			}

		case "cookie":
			c, _ := req.Cookie(param.name)
			if c != nil {
				typ := reflect.TypeOf(value)
				switch typ.String() {
				case cookieTypeString:
					value = c
				case cookieTypeString2:
					value = &c
				}
			} else if param.IsRequired() {
				return param.Error("missing cookie param")
			}
		}
		if err = param.makeVerifyRules(); err != nil {
			return err
		}
		if err = validation.Validate(value, param.rules...); err != nil {
			return err
		}
	}
	return
}

// Params gets the parameter information
func (a *Api) Params() []*Param {
	return a.params
}

// BindByName binds the net/http api params to a new struct and validate it.
func BindByName(
	apiName string,
	req *http.Request,
	apiParams ParamsKVStore,
) (
	interface{},
	error,
) {
	api, err := GetApi(apiName)
	if err != nil {
		return nil, err
	}
	return api.BindNew(req, apiParams)
}

// Bind binds the net/http api params to the `structPointer` param and validate it.
// note: structPointer must be struct pointea.
func Bind(
	structPointer interface{},
	req *http.Request,
	apiParams ParamsKVStore,
) error {
	api, err := GetApi(reflect.TypeOf(structPointer).String())
	if err != nil {
		return err
	}
	return api.BindAt(structPointer, req, apiParams)
}

func (apiMap *ApiMap) get(apiName string) (*Api, bool) {
	apiMap.RLock()
	defer apiMap.RUnlock()
	api, ok := apiMap.Map[apiName]
	return api, ok
}

func (apiMap *ApiMap) set(api *Api) {
	apiMap.Lock()
	apiMap.Map[api.name] = api
	defer apiMap.Unlock()
}

// Get distinct and sorted parameters information.
func distinctAndSortedApiParams(infos []APIParam) []APIParam {
	infoMap := make(map[string]APIParam, len(infos))
	ks := make([]string, 0, len(infos))
	for _, info := range infos {
		k := info.Name + "<\r-\n-\t>" + info.In
		ks = append(ks, k)
		// Filter duplicate parameters, and maximize access to information.
		if newinfo, ok := infoMap[k]; ok {
			if !newinfo.Required && info.Required {
				newinfo.Required = info.Required
			}
			if len(newinfo.Desc) == 0 && len(info.Desc) > 0 {
				newinfo.Desc = info.Desc
			}
			infoMap[k] = newinfo
			continue
		}
		infoMap[k] = info
	}
	sort.Strings(ks)
	newinfos := make([]APIParam, 0, len(ks))
	for _, k := range ks {
		newinfos = append(newinfos, infoMap[k])
	}
	return newinfos
}
