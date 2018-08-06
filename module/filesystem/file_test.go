package filesystem

import (
	"fmt"
)

func ExampleFileHandler() {
	var handler Handler = NewFileHandler()
	handler.Initiate(nil)
	handler.SetContent("1", []byte{1, 2, 3})
	fmt.Println(handler.GetContent("1"))
	// Output:
	// [1 2 3] <nil>
}
