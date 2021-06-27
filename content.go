package content

import (
	"io"
	"io/fs"
)

// EOF is set to io.EOF
var EOF = io.EOF

// ErrNotExist is set to fs.ErrNotExist
var ErrNotExist = fs.ErrNotExist

// FIXME: should we just be exposing this as an fs.FS???
// Yes - just use fs.FS!  we can add more interfaces as needed to provide
// additional/more specific functionality

/*
Changes:
- what about the "metadata" associated with a page? includes things
  like title and desc, but also which template to use!  How and where do we store this?
  Examine the options like in HTML comment, top header section (like hugo),
  or separate file
- while we're at it, let's please solve the issue of machine-readable
  config that preserves comments!
*/

// // P represents a Page or Part.
// type P interface {
// 	Path() string
// 	SetPath(v string)
// 	ContentType() string
// 	SetContentType(v string)
// 	Contents() string
// 	SetContents(v string)
// }

// type PageStore interface {
// 	WritePage(p P) error
// 	ReadPage(path string, p P) error
// 	DeletePage(path string) error
// 	MovePage(fromPath, toPath string) error
// 	IteratePages(dir string) (PageIterator, error)
// }

// type PageIterator interface {
// 	NextPage(p P) error
// 	NextPath() (string, error)
// }

// type PartStore interface {
// 	WritePart(p P) error
// 	ReadPart(path string, p P) error
// 	DeletePart(path string) error
// 	IterateParts(dir string) (PartIterator, error)
// }

// type PartIterator interface {
// 	NextPart(p P) error
// 	NextPath() (string, error)
// }
