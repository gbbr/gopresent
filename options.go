package gopresent

import (
	"fmt"
	"go/build"
	"log"
	"os"
	"os/user"
	"path/filepath"
)

// thisPackage defines the name of this package.
const thisPackage = "github.com/gbbr/gopresent"

// Options allows to optionally customize the behaviour of the application.
type Options struct {
	// WebRoot specifies the path where the templates and static files are to be found.
	// If empty, $GOPATH/src/github.com/gbbr/gopresent will be used as a default.
	WebRoot string

	// StorageRoot specifies the path used for storage. A folder called ".gopresent" will be
	// created at this path. If empty, it defaults to the user's home directory.
	StorageRoot string

	// MaxFileSize specifies the maximum file size allowed for an uploaded slide. It defaults
	// to 100Kb.
	MaxFileSize int64

	// MaxHours specifies the number of hours after which a slide will be considered expired
	// and will be evicted.
	MaxHours int

	// MaxQuota specifies the maximum disk quota allowed to the application. If it is surpassed
	// no more files will be uploaded and an error will be returned. It defaults to 100Mb.
	MaxQuota int64
}

// prepare applies the default values to any unset Options fields.
func (opts *Options) prepare() {
	if opts.WebRoot == "" {
		p, err := build.Default.Import(thisPackage, "", build.FindOnly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Couldn't find gopresent files: %v\n", err)
			os.Exit(1)
		}
		opts.WebRoot = p.Dir
	}
	if opts.StorageRoot == "" {
		u, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}
		if u.HomeDir == "" {
			log.Fatal("could not obtain home dir")
		}
		storagePath := filepath.Join(u.HomeDir, ".gopresent")
		err = os.MkdirAll(filepath.Join(storagePath, "slides"), os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		opts.StorageRoot = storagePath
	}
	if opts.MaxFileSize == 0 {
		opts.MaxFileSize = defaultMaxFileSize
	}
	if opts.MaxHours == 0 {
		opts.MaxHours = defaultMaxHours
	}
	if opts.MaxQuota == 0 {
		opts.MaxQuota = defaultMaxQuota
	}
}
