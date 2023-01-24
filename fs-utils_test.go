package pomegranate

import (
	"io"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFSUtils_Open(t *testing.T) {
	tests := []struct {
		name     string
		f        fs.ReadDirFS
		path     string
		contents []byte
		wantErr  bool
	}{
		{"errors opening non-existant file", OsDir("fixtures/embed"), "not-a-dir", []byte{}, true},
		{"open real file", OsDir("fixtures/embed"), "empty", []byte{}, false},
		{"using embedded file", sub{"fixtures/embed", embedded}, "empty", []byte{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.Open(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("OsDir.Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			payload, err := io.ReadAll(got)
			assert.Nil(t, err)
			assert.Equal(t, tt.contents, payload)
		})
	}
}

func TestFSUtils_ReadDir(t *testing.T) {
	tests := []struct {
		name    string
		f       fs.ReadDirFS
		path    string
		want    []fs.DirEntry
		wantErr bool
	}{
		{"error getting non-existant path", OsDir("fixtures/not-a-real-path"), "doesnt-matter", nil, true},
		{"zero-sized file", OsDir("fixtures"), "embed", nil, false},
		{"embedded", sub{"fixtures", embedded}, "embed", nil, false},
		{"same as above, but using FromEmbed", FromEmbed(embedded, "fixtures"), "embed", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.ReadDir(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("OsDir.ReadDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			assert.NotNilf(t, got, "We want to ensure the the response is not nil")
		})
	}
}
