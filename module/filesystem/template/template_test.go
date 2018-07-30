package template

import (
	"fmt"
)

func ExampleFileTemplateHandler() {
	var handler TemplateHandler = NewFileTemplateHandler()
	handler.Initiate(nil)
	handler.SetContent("1", []byte{1, 2, 3})
	fmt.Println(handler.GetContent("1"))
	// Output:
	// [1 2 3] <nil>
}
