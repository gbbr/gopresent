package gopresent

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/tools/present"
)

func init() {
	present.NotesEnabled = true
}

// handleSlide is the handler for the page serving slides.
func (a *app) handleSlide(w http.ResponseWriter, r *http.Request) error {
	key := path.Base(r.URL.Path)
	if len(key) == 0 {
		return errors.New("not found")
	}
	content, err := ioutil.ReadFile(filepath.Join(a.opts.StorageRoot, "slides", key))
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
