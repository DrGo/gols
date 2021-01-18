package gols

import (
	"fmt"
	"net/http"
	"path/filepath"
)

type FS struct {
	fs http.FileSystem
  root string
  entry string
}

func (anfs FS) Open(path string) (http.File, error) {
	f, err := anfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not open file %s: %v", path, err)
	}
	if s.IsDir() {
		index := filepath.Join(path, anfs.entry)
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
