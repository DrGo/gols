package gols

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

type FS struct {
	fs http.FileSystem
  root string
  entry string
  AllowDotFiles bool
}

func (anfs FS) Open(path string) (http.File, error) {
	if !anfs.AllowDotFiles && hasDotPrefix(path) {
	  return nil, fmt.Errorf("forbidden")
	}
	f, err := anfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not open file %s: %v", path, err)
	}
	// if dir, return dir/index.html
	if s.IsDir() {
		index := filepath.Join(path, "index.html")
		if _, err := anfs.fs.Open(index); err != nil {
			closeErr := f.Close()
			if closeErr != nil {
				return nil, closeErr
			}

			return nil, err
		}
	}

	return f, nil
}

// Check if path contains a dotfile. Source: https://pkg.go.dev/net/http#example-FileServer-DotFileHiding
func hasDotPrefix(path string) bool {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
