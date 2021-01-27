package gols

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattetti/filebuffer"
)

var _ http.FileSystem = (*FS)(nil)

// FS wraps an http.FileSystem (e.g., from http.Dir()) to permit
// finer control over serving files
type FS struct {
	fs    http.FileSystem
	root  string
	entry string
	// if false(default), attempts to access files with dots anywhere in their path return 404 (Not found) error
	AllowDotFiles bool
	// if false(default), attempts to navigate to a dir return 404 error instead of dir listing
	AllowDirListing bool
	//
	BeforeServing func(hf http.File, name string, mode fs.FileMode) (http.File, error)
	AfterServing  func(hf http.File, name string, mode fs.FileMode) (http.File, error)
}

// Open rigged to disable dir listing
func (anfs FS) Open(path string) (http.File, error) {
	fullPath := filepath.Clean(filepath.Join(anfs.root, path))
	if filepath.Ext(path) == ".html" {
	  fmt.Println("opening:" + fullPath)
  }  
	if !anfs.AllowDotFiles && hasDotPrefix(path) {
		return nil, fmt.Errorf("forbidden")
	}
	f, err := anfs.fs.Open(path)
	if err != nil {
		fmt.Println("fs.open:", err)
		return nil, err
	}
	// defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("could not open file %s: %v", path, err)
	}
	// if dir and dir/index.html fails to open return an error
	// resulting in 404
	if !anfs.AllowDirListing && fi.IsDir() {
		index := filepath.Join(path, "index.html")
		if _, err := anfs.fs.Open(index); err != nil {
			f.Close()
			return nil, err
		}
	}
	if anfs.BeforeServing != nil {
		modf, err := anfs.BeforeServing(f, fullPath, fi.Mode())
		if err != nil {
			fmt.Println(err)
			return nil, err
		} else if modf != nil { //skipped eg not a file
			if anfs.AfterServing != nil {
				defer anfs.AfterServing(modf, fullPath, fi.Mode())
			}
			return modf, nil
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

type File struct {
	*filebuffer.Buffer
	name string
	mode fs.FileMode
}


func NewFile(buf []byte, name string, mode fs.FileMode) *File {
	f := filebuffer.New(buf)
	return &File{f, name, mode}
}

func NewFromReader(r io.Reader, name string, mode fs.FileMode) (*File, error) {
	f, err := filebuffer.NewFromReader(r)
	if err != nil {
		return nil, err
	}
	return &File{f, name, mode}, nil
}

func (f *File) Size() int64 {
	return int64(f.Buff.Len())
}

func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	fmt.Println("readdir: " + f.name)
	return nil, nil
}

func (f *File) Stat() (fs.FileInfo, error) { return &fileInfo{f}, nil }

type fileInfo struct {
	f *File
}

func (i *fileInfo) Name() string               { return i.f.name }
func (i *fileInfo) Size() int64                { return i.f.Size() }
func (i *fileInfo) Type() fs.FileMode          { return i.f.mode.Type() }
func (i *fileInfo) ModTime() time.Time         { return time.Now() }
func (i *fileInfo) IsDir() bool                { return i.f.mode&fs.ModeDir != 0 }
func (i *fileInfo) Sys() interface{}           { return nil }
func (i *fileInfo) Info() (fs.FileInfo, error) { return i, nil }
func (i *fileInfo) Mode() fs.FileMode          { return i.f.mode }
