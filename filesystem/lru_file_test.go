package filesystem

import (
	"fmt"
	"time"

	"github.com/ltick/tick-framework/config"
)

func ExampleFileHandler() {
	var configs map[string]config.Option = map[string]config.Option{
		"FILESYSTEM_PROVIDER":                config.Option{Type: config.String, Default: "file", EnvironmentKey: "FILESYSTEM_PROVIDER"},
		"FILESYSTEM_DEFRAG_CONTENT_INTERVAL": config.Option{Type: config.Duration, Default: 30 * time.Minute, EnvironmentKey: "FILESYSTEM_DEFRAG_CONTENT_INTERVAL"},
		"FILESYSTEM_DEFRAG_CONTENT_LIFETIME": config.Option{Type: config.Duration, Default: 24 * time.Hour, EnvironmentKey: "FILESYSTEM_DEFRAG_CONTENT_LIFETIME"},
		"FILESYSTEM_LRU_CAPACITY":            config.Option{Type: config.Int64, Default: 32 * 1024 * 1024, EnvironmentKey: "FILESYSTEM_LRU_CAPACITY"},
	}
	var config *config.Config = config.NewConfig()
	config.Initiate(nil)
	config.SetOptions(nil, configs)
	var handler Handler = NewFileHandler()
	handler.Initiate(nil, config)
	handler.SetContent("1", []byte{1, 2, 3})
	fmt.Println(handler.GetContent("1"))
	// Output:
	// [1 2 3] <nil>
}
