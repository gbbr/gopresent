package gopresent

// TODO: reduce filepath.Join
// TODO: improve errors

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"go/build"
	"html/template"
	"io"
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
	maxFileSize = 1e5 // 100K
	maxHours    = 48
	maxQuota    = 1e8 // 100M
)

type app struct {
	pages       *template.Template
	present     *template.Template
	mux         *http.ServeMux
	storagePath string
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *app) writeTemplate(w io.Writer, name string, data interface{}) error {
	var out bytes.Buffer
	err := a.pages.ExecuteTemplate(&out, name, data)
	if err != nil {
		return err
	}
	_, err = w.Write(out.Bytes())
	return err
}

func (a *app) removeExpired() error {
	return filepath.Walk(filepath.Join(a.storagePath, "slides"), func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		if time.Since(fi.ModTime()) > maxHours*time.Hour {
			if err := os.Remove(path); err != nil {
				log.Println(err)
			}
		}
		return nil
	})
}

func (a *app) index(w http.ResponseWriter, r *http.Request) error {
	return a.writeTemplate(w, "index", nil)
}

func (a *app) getSlide(key string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(a.storagePath, "slides", key))
}

func (a *app) writeSlide(key string, data []byte) error {
	slidePath := filepath.Join(a.storagePath, "slides")
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

func (a *app) slide(w http.ResponseWriter, r *http.Request) error {
	key := path.Base(r.URL.Path)
	if len(key) == 0 {
		return errors.New("not found")
	}
	content, err := a.getSlide(key)
	if os.IsNotExist(err) {
		return errors.New("not found")
	}
	if err != nil {
		return err
	}
	doc, err := present.Parse(bytes.NewReader(content), key, 0)
	if err != nil {
		return err
	}
	return doc.Render(w, a.present)
}

func (a *app) upload(w http.ResponseWriter, r *http.Request) error {
	f, h, err := r.FormFile("filename")
	defer f.Close()
	if err != nil {
		return err
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
		// we write again, even if it exists, to update ModTime
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

func withError(fn func(w http.ResponseWriter, r *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

func Handler() http.Handler {
	p, err := build.Default.Import("github.com/gbbr/gopresent", "", build.FindOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't find gopresent files: %v\n", err)
		os.Exit(1)
	}
	baseDir := p.Dir
	a := &app{
		pages: template.Must(template.New("gopresent.io").ParseFiles(
			filepath.Join(baseDir, "/templates/index.tmpl"),
		)),
		present: template.Must(present.Template().ParseFiles(
			filepath.Join(baseDir, "/templates/action.tmpl"),
			filepath.Join(baseDir, "/templates/slides.tmpl"),
		)),
		mux: http.NewServeMux(),
	}
	a.mux.Handle("/", withError(a.index))
	a.mux.Handle("/upload", withError(a.upload))
	a.mux.Handle("/slide/", withError(a.slide))
	a.mux.Handle("/static/", http.FileServer(http.Dir(baseDir)))

	u, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	if u.HomeDir == "" {
		// n/a
		log.Fatal("could not obtain home dir")
	}
	a.storagePath = filepath.Join(u.HomeDir, ".gopresent")
	err = os.MkdirAll(filepath.Join(a.storagePath, "slides"), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	err = os.MkdirAll(filepath.Join(a.storagePath, "ips"), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		if err := a.removeExpired(); err != nil {
			log.Println(err)
		}
		for {
			select {
			case <-time.Tick(maxHours * time.Hour):
				if err := a.removeExpired(); err != nil {
					log.Println(err)
				}
			}
		}
	}()

	log.Printf("Storage initialized at: %s\n", a.storagePath)

	return a
}
