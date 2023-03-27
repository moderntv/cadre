package file

import (
	"fmt"
	"io"
	"os"

	"github.com/moderntv/cadre/config/encoder"
	"github.com/moderntv/cadre/config/source"
)

var (
	Name               = "file"
	_    source.Source = &FileSource{}
)

type FileSource struct {
	path    string
	encoder encoder.Encoder
}

func NewSource(path string, encoder encoder.Encoder) (fs *FileSource, err error) {
	fs = &FileSource{
		path:    path,
		encoder: encoder,
	}

	return
}

func (fs *FileSource) Name() string {
	return Name
}

func (fs *FileSource) Read() (d []byte, err error) {
	h, err := os.Open(fs.path)
	if err != nil {
		return
	}
	defer h.Close()

	d, err = io.ReadAll(h)
	if err != nil {
		return
	}

	return
}

func (fs *FileSource) Save(dst any) (err error) {
	data, err := fs.encoder.Encode(dst)
	if err != nil {
		return
	}

	f, err := os.Create(fs.path)
	if err != nil {
		return
	}

	defer f.Close()
	_, err = f.Write(data)
	if err != nil {
		return
	}

	return nil
}

func (fs *FileSource) Load(dst any) (err error) {
	d, err := fs.Read()
	if err != nil {
		return fmt.Errorf("data read failed: %w", err)
	}

	return fs.encoder.Decode(d, dst)
}

func (fs *FileSource) Watch() (w source.Watcher, err error) {
	w, err = newWatcher(fs.path)

	return
}
