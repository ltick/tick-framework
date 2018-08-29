package writer

import (
	"fmt"
	"net/http"
	"encoding/json"

	"github.com/ltick/tick-routing"
)

type ErrorData struct {
	*Data
}

func (this *ErrorData) Error() string {
	return this.Message
}
func (this *ErrorData) StatusCode() int {
	return this.Status
}
func (this *ErrorData) ErrorCode() string {
	return this.Code
}

type ErrorDataWriter struct{}

func (w *ErrorDataWriter) SetHeader(res http.ResponseWriter) {
	res.Header().Set("Access-Control-Allow-Origin", "*")
	res.Header().Set("Content-Type", "application/json")
}

func (w *ErrorDataWriter) Write(res http.ResponseWriter, data interface{}) (err error) {
	switch data.(type) {
	case []byte:
		byte := data.([]byte)
		_, err = res.Write(byte)
	case string:
		byte := []byte(data.(string))
		_, err = res.Write(byte)
	case *ErrorData:
		errorData, ok := data.(*ErrorData)
		if !ok {
			return routing.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("get audio: data type error"))
		}
		errorDataBody, err := json.Marshal(errorData)
		if err != nil {
			return routing.NewHTTPError(errorData.StatusCode(), errorData.ErrorCode()+":"+errorData.Error())
		}
		w.SetHeader(res)
		return routing.NewHTTPError(errorData.StatusCode(), string(errorDataBody))
	default:
		if data != nil {
			_, err = fmt.Fprint(res, data)
			return err
		}
	}
	return err
}

