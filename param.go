package ltick

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/juju/errors"
	"github.com/ltick/ltick-validation"
)

const (
	fileTypeString    = "*multipart.FileHeader"
	fileTypeString2   = "multipart.FileHeader"
	filesTypeString   = "[]*multipart.FileHeader"
	filesTypeString2  = "[]multipart.FileHeader"
	cookieTypeString  = "*http.Cookie"
	cookieTypeString2 = "http.Cookie"
	// fasthttpCookieTypeString = "fasthttp.Cookie"
	stringTypeString = "string"
	bytesTypeString  = "[]byte"
	bytes2TypeString = "[]uint8"
)

// some define
const (
	TAG_PARAM        = "param"    // request param tag name
	TAG_IGNORE_PARAM = "-"        // ignore request param tag value
	KEY_IN           = "in"       // position of param
	KEY_NAME         = "name"     // specify request param`s name
	KEY_REQUIRED     = "required" // request param is required or not
	KEY_DESC         = "desc"     // request param description
	KEY_LEN          = "len"      // length range of param's value
	KEY_RANGE        = "range"    // numerical range of param's value
	KEY_NOTEMPTY     = "notempty" // param`s value can not be zero
	KEY_REGEXP       = "regexp"   // verify the value of the param with a regular expression(param value can not be null)
	KEY_MAXMB        = "maxmb"    // when request Content-Type is multipart/form-data, the max memory for body.(multi-param, whichever is greater)
	KEY_ERR          = "err"      // the custom error for binding or validating

	MB                 = 1 << 20 // 1MB
	defaultMaxMemory   = 32 * MB // 32 MB
	defaultMaxMemoryMB = 32
)

var (
	// TagInValues is values for tag 'in'
	TagInValues = map[string]bool{
		"path":     true,
		"query":    true,
		"formData": true,
		"body":     true,
		"header":   true,
		"cookie":   true,
	}
)

type (
	// Param use the struct field to define a request parameter model
	Param struct {
		apiName    string // Api name
		name       string // param name
		indexPath  []int
		isRequired bool              // file is required or not
		isFile     bool              // is file param or not
		tags       map[string]string // struct tags for this param
		rules      []validation.Rule
		rawTag     reflect.StructTag // the raw tag
		rawValue   reflect.Value     // the raw tag value
		err        error             // the custom error for binding or validating
	}
	// ParamNameMapper maps param name from struct param name
	ParamNameMapper func(fieldName string) (paramName string)
)

// Raw gets the param's original value
func (param *Param) Raw() interface{} {
	return param.rawValue.Interface()
}

// APIName gets ParamsAPI name
func (param *Param) APIName() string {
	return param.apiName
}

// Name gets parameter field name
func (param *Param) Name() string {
	return param.name
}

// In get the type value for the param
func (param *Param) In() string {
	return param.tags[KEY_IN]
}

// IsRequired tests if the param is declared
func (param *Param) IsRequired() bool {
	return param.isRequired
}

// Description gets the description value for the param
func (param *Param) Description() string {
	return param.tags[KEY_DESC]
}

// IsFile tests if the param is type *multipart.FileHeader
func (param *Param) IsFile() bool {
	return param.isFile
}

func (param *Param) Error(reason string) error {
	if param.err != nil {
		return param.err
	}
	return errors.Trace(fmt.Errorf("%s|%s|%s", param.apiName, param.name, reason))
}

func (param *Param) makeVerifyRules() (err error) {
	defer func() {
		p := recover()
		if p != nil {
			err = fmt.Errorf("%v", p)
		}
	}()
	param.rules = make([]validation.Rule, 0)
	// length
	if tuple, ok := param.tags[KEY_LEN]; ok {
		var a, b = parseTuple(tuple)
		var min, max int
		var err error
		if len(a) > 0 {
			min, err = strconv.Atoi(a)
			if err != nil {
				return err
			}
		}
		if len(b) > 0 {
			max, err = strconv.Atoi(b)
			if err != nil {
				return err
			}
		}
		param.rules = append(param.rules, validation.Length(min, max))
	}
	// range
	if tuple, ok := param.tags[KEY_RANGE]; ok {
		var a, b = parseTuple(tuple)
		var min, max float64
		var err error
		if len(a) > 0 {
			min, err = strconv.ParseFloat(a, 64)
			if err != nil {
				return err
			}
		}
		if len(b) > 0 {
			max, err = strconv.ParseFloat(b, 64)
			if err != nil {
				return err
			}
		}
		param.rules = append(param.rules, validation.Range(min, max))
	}
	// notempty
	if _, ok := param.tags[KEY_NOTEMPTY]; ok {
		param.rules = append(param.rules, validation.NotEmpty)
	}
	// regexp
	if reg, ok := param.tags[KEY_REGEXP]; ok {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		param.rules = append(param.rules, validation.Match(re))
	}
	return
}

// ParseTags returns the key-value in the tag string.
// If the tag does not have the conventional format,
// the value returned by ParseTags is unspecified.
func ParseTags(tag string) map[string]string {
	var values = map[string]string{}

	for tag != "" {
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] != '<' {
			i++
		}
		if i >= len(tag) || tag[i] != '<' {
			break
		}
		i++

		// Skip the left Spaces
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		if i >= len(tag) {
			break
		}

		tag = tag[i:]
		if tag == "" {
			break
		}

		var name, value string
		var hadName bool
		i = 0
	PAIR:
		for i < len(tag) {
			switch tag[i] {
			case ':':
				if hadName {
					i++
					continue
				}
				name = strings.TrimRight(tag[:i], " ")
				tag = strings.TrimLeft(tag[i+1:], " ")
				hadName = true
				i = 0
			case '\\':
				i++
				// Fix the escape character of `\\<` or `\\>`
				if tag[i] == '<' || tag[i] == '>' {
					tag = tag[:i-1] + tag[i:]
				} else {
					i++
				}
			case '>':
				if !hadName {
					name = strings.TrimRight(tag[:i], " ")
				} else {
					value = strings.TrimRight(tag[:i], " ")
				}
				values[name] = value
				break PAIR
			default:
				i++
			}
		}
		if i >= len(tag) {
			break
		}
		tag = tag[i+1:]
	}
	return values
}

func parseTuple(tuple string) (string, string) {
	c := strings.Split(tuple, ":")
	var a, b string
	switch len(c) {
	case 1:
		a = c[0]
		if len(a) > 0 {
			return a, a
		}
	case 2:
		a = c[0]
		b = c[1]
		if len(a) > 0 || len(b) > 0 {
			return a, b
		}
	}
	panic("invalid validation tuple")
}

func validateNonZero() (func(value reflect.Value) error, error) {
	return func(value reflect.Value) error {
		obj := value.Interface()
		if obj == reflect.Zero(value.Type()).Interface() {
			return errors.New("not set")
		}
		return nil
	}, nil
}

func validateRegexp(isStrings bool, reg string) (func(value reflect.Value) error, error) {
	re, err := regexp.Compile(reg)
	if err != nil {
		return nil, err
	}
	if !isStrings {
		return func(value reflect.Value) error {
			s := value.String()
			if !re.MatchString(s) {
				return fmt.Errorf("not match %s: %s", reg, s)
			}
			return nil
		}, nil
	} else {
		return func(value reflect.Value) error {
			for _, s := range value.Interface().([]string) {
				if !re.MatchString(s) {
					return fmt.Errorf("not match %s: %s", reg, s)
				}
			}
			return nil
		}, nil
	}
}
