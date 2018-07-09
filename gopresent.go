package gopresent

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"go/build"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/tools/present"
)

const (
	// maxFileSize specifies the maximum allowed file size for a slide.
	maxFileSize = 1e5 // 100K

	// checkFrequency specifies the frequency at which the storage will be checked
	// for expired slides
	checkFrequency = time.Hour

	// maxHours defines the maximum number of hours before a slide is deleted.
	maxHours = 48

	// maxQuota defines the maximum disk quota that the app is allowed to use.
	maxQuota = 1e8 // 100M
)

// thisPackage defines the name of this package.
const thisPackage = "github.com/gbbr/gopresent"

// app is the gopresent.io application.
type app struct {
	// opts holds a set of options used to configure the app.
	opts *Options

	// pages holds the templates used to render the HTML pages.
	pages *template.Template

	// present holds the templates used to render slides.
	present *template.Template
}

// writeTemplate writes the template specifies by name to w using the given data. It returns an error
// if it fails. It does not write anything to w on error.
func (a *app) writeTemplate(w http.ResponseWriter, name string, data interface{}) error {
	var out bytes.Buffer
	// write to a buffer first to avoid writing the HTTP headers in case of an error
	err := a.pages.ExecuteTemplate(&out, name, data)
	if err != nil {
		return err
	}
	// succeeded, attempt to write the output
	_, err = w.Write(out.Bytes())
	return err
}

// checkExpired starts a loop which occasionally checks for expired slides and
// deletes them.
func (a *app) checkExpired() {
	rm := func() {
		if err := a.removeExpired(); err != nil {
			log.Println(err)
		}
	}
	rm()
	for {
		select {
		case <-time.Tick(checkFrequency):
			rm()
		}
	}
}

// removeExpired removes all expired slides from the storage.
func (a *app) removeExpired() error {
	return filepath.Walk(filepath.Join(a.opts.StorageRoot, "slides"), func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			// cannot continue
			return err
		}
		if fi.IsDir() {
			// skip
			return nil
		}
		if time.Since(fi.ModTime()) > maxHours*time.Hour {
			if err := os.Remove(path); err != nil {
				log.Println(err)
				// we've failed deleting this file, but we should try
				// and continue nevertheless
			}
		}
		return nil
	})
}

// getSlide returns the slide having the given key.
func (a *app) getSlide(key string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(a.opts.StorageRoot, "slides", key))
}

// writeSlide writes the slide with the given key, containing the given data to disk.
func (a *app) writeSlide(key string, data []byte) error {
	slidePath := filepath.Join(a.opts.StorageRoot, "slides")
	fi, err := os.Stat(slidePath)
	if err != nil {
		return err
	}
	if fi.Size() > maxQuota {
		log.Printf("disk quota exceeded: %d > %d\n", fi.Size(), int(maxQuota))
		return errors.New("disk quota exceeded")
	}
	return ioutil.WriteFile(filepath.Join(slidePath, key), data, os.ModePerm)
}

// handleIndex is the handler for the root page.
func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) error {
	return a.writeTemplate(w, "index", nil)
}

// handleSlide is the handler for the page serving slides.
func (a *app) handleSlide(w http.ResponseWriter, r *http.Request) error {
	key := path.Base(r.URL.Path)
	if len(key) == 0 {
		return errors.New("not found")
	}
	content, err := a.getSlide(key)
	if os.IsNotExist(err) {
		return errors.New("not found")
	}
	if err != nil {
		log.Printf("error reading slide: %v", err)
		return errors.New("not found")
	}
	doc, err := present.Parse(bytes.NewReader(content), key, 0)
	if err != nil {
		log.Printf("error parsing slide: %v", err)
		return errors.New("parse error")
	}
	err = doc.Render(w, a.present)
	if err != nil {
		log.Printf("error rendering slide: %v", err)
		return errors.New("render error")
	}
	return nil
}

// handleUpload is the handler for the upload page.
func (a *app) handleUpload(w http.ResponseWriter, r *http.Request) error {
	f, h, err := r.FormFile("filename")
	defer f.Close()
	if err != nil {
		return errors.New("not found")
	}
	if h.Size > maxFileSize {
		return errors.New("maximum file size is 100KB")
	}
	slurp, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%x", md5.Sum(slurp))
	_, err = present.Parse(bytes.NewReader(slurp), key, 0)
	if err != nil {
		return err
	}
	if err := a.writeSlide(key, slurp); err != nil {
		// even if it exists, we write again to update ModTime
		return err
	}
	return a.writeTemplate(w, "upload", struct {
		URL   string
		Hours string
	}{
		URL:   fmt.Sprintf("https://gopresent.io/slide/%s", key),
		Hours: strconv.Itoa(maxHours),
	})
}

// withError returns a new http.Handler which uses fn to handle a request. If fn returns an
// error, it will use it as an HTTP error for the user.
func withError(fn func(w http.ResponseWriter, r *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err != nil {
			// TODO: this HTTP code is incorrect in some scenarios
			code := http.StatusInternalServerError
			http.Error(w, err.Error(), code)
			return
		}
	})
}

// Options allows to optionally customize the behaviour of the application.
type Options struct {
	// WebRoot specifies the path where the templates and static files are to be found.
	// If empty, $GOPATH/src/github.com/gbbr/gopresent will be used as a default.
	WebRoot string

	// StorageRoot specifies the path used for storage. A folder called ".gopresent" will be
	// created at this path. If empty, it defaults to the user's home directory.
	StorageRoot string
}

// NewApp returns an http.Handler that will serve the app. An empty set of options
// can (and usually should) be provided in order to use the defaults.
func NewApp(opts Options) http.Handler {
	if opts.WebRoot == "" {
		// web root not set, try default
		p, err := build.Default.Import(thisPackage, "", build.FindOnly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Couldn't find gopresent files: %v\n", err)
			os.Exit(1)
		}
		opts.WebRoot = p.Dir
	}
	if opts.StorageRoot == "" {
		// storage not set, try default
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
	a := &app{
		opts: &opts,
		pages: template.Must(template.New("gopresent.io").ParseFiles(
			filepath.Join(opts.WebRoot, "/templates/index.tmpl"),
		)),
		present: template.Must(present.Template().ParseFiles(
			filepath.Join(opts.WebRoot, "/templates/action.tmpl"),
			filepath.Join(opts.WebRoot, "/templates/slides.tmpl"),
		)),
	}

	go a.checkExpired()

	log.Printf("Running with storage: %s\n", a.opts.StorageRoot)

	mux := http.NewServeMux()
	mux.Handle("/", withError(a.handleIndex))
	mux.Handle("/upload", withError(a.handleUpload))
	mux.Handle("/slide/", withError(a.handleSlide))
	mux.Handle("/static/", http.FileServer(http.Dir(opts.WebRoot)))

	return mux
}
