package gopresent

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/tools/present"
)

// checkFrequency specifies the frequency at which the storage will be checked
// for expired slides
const checkFrequency = time.Hour

// app is the gopresent.io application.
type app struct {
	// opts holds a set of options used to configure the app.
	opts *Options

	// pages holds the templates used to render the HTML pages.
	pages *template.Template

	// present holds the templates used to render slides.
	present *template.Template
}

// NewApp returns an http.Handler that will serve the app. An empty set of options
// can (and usually should) be provided in order to use the defaults.
func NewApp(opts Options) http.Handler {
	opts.prepare()

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
		if time.Since(fi.ModTime()) > time.Duration(a.opts.MaxHours)*time.Hour {
			if err := os.Remove(path); err != nil {
				log.Println(err)
				// we've failed deleting this file, but we should try
				// and continue nevertheless
			}
		}
		return nil
	})
}

// handleIndex is the handler for the root page.
func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) error {
	return a.serveTemplate(w, "index", nil)
}
