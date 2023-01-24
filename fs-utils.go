package pomegranate

import (
	"embed"
	"io/fs"
	"os"
	"path"
)

// A OsDir is a fs.ReadDirFS that expects to be sitting on the local file system
type OsDir string

// Openb implements [fs.ReadDirFS] and just proxies for os.Open.
// This is not safe with "." and "..", and is intended only for well known paths
func (osd OsDir) Open(name string) (fs.File, error) {
	return os.Open(path.Join(string(osd), name))
}

// ReadDir implemnts [fs.ReadDirFS] and just proxies for [os.ReadDir]
// This is not safe with "." and "..", and is intended only for well known paths
func (osd OsDir) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(path.Join(string(osd), name))
}

var _ = fs.ReadDirFS(OsDir(""))

type sub struct {
	prefix string
	f      embed.FS
}

// Open conforms to [fs.ReadDirFS]
func (s sub) Open(name string) (fs.File, error) {
	return s.f.Open(path.Join(s.prefix, name))
}

// ReadDir implemnts [fs.ReadDirFS] and just proxies for [os.ReadDir]
func (s sub) ReadDir(name string) ([]fs.DirEntry, error) {
	return s.f.ReadDir(path.Join(s.prefix, name))
}

/*
	FromEmbed is able to convert a embed.FS into something more useful.

When you do

		//go:embed some/pathlike/to/migrations
	  var embedded embed.FS

You must prefix every call to embedded with the string "some/path/or/migrations",
or use this like


		//go:embed some/pathlike/to/migrations
	  var embedded embed.FS
    var migrations = FromEmbed(embedded, "some/pathlike/to/migrations")

Which allows one to directly pass `migrations` into
*/

func FromEmbed(f embed.FS, prefix string) fs.ReadDirFS { return sub{prefix: prefix, f: f} }
