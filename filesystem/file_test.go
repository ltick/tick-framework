package filesystem

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ltick/tick-framework/config"
)

var (
	handler Handler = NewFileHandler()
)

func init() {
	var configs map[string]config.Option = map[string]config.Option{
		"FILESYSTEM_PROVIDER":                config.Option{Type: config.String, Default: "file", EnvironmentKey: "FILESYSTEM_PROVIDER"},
		"FILESYSTEM_DEFRAG_CONTENT_INTERVAL": config.Option{Type: config.Duration, Default: 30 * time.Minute, EnvironmentKey: "FILESYSTEM_DEFRAG_CONTENT_INTERVAL"},
		"FILESYSTEM_DEFRAG_CONTENT_LIFETIME": config.Option{Type: config.Duration, Default: 24 * time.Hour, EnvironmentKey: "FILESYSTEM_DEFRAG_CONTENT_LIFETIME"},
		"FILESYSTEM_LRU_CAPACITY":            config.Option{Type: config.Int64, Default: 32 * 1024 * 1024, EnvironmentKey: "FILESYSTEM_LRU_CAPACITY"},
	}

	var config *config.Config = config.NewConfig()
	config.Initiate(nil)
	config.SetOptions(nil, configs)
	handler.Initiate(nil, config)
}

func ExampleFileHandler() {
	handler.SetContent("1", []byte{1, 2, 3})
	fmt.Println(handler.GetContent("1"))
	// Output:
	// [1 2 3] <nil>
}

func BenchmarkFileSetContent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		handler.SetContent("abc", []byte{1, 2, 3})
	}
}

func BenchmarkFileGetContent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		handler.GetContent("abc")
	}
}

func BenchmarkFileSetContentParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			handler.SetContent("abc", []byte{1, 2, 3})
		}
	})
}

func BenchmarkFileGetContentParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			handler.GetContent("abc")
		}
	})
}

func BenchmarkFileRandomSetContent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		handler.SetContent(fmt.Sprintf("%d", rand.Uint64()), bytes.Repeat([]byte("abcdef"), rand.Intn(1200)))
	}
}
