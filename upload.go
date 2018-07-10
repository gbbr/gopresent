package gopresent

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/tools/present"
)

// handleUpload handles the /upload endpoint, writes any received file to disk
// and greets the user with the upload URL. It also validates that the received
// file is a valid parsable slide.
func (a *app) handleUpload(w http.ResponseWriter, r *http.Request) error {
	f, h, err := r.FormFile("filename")
	if err != nil {
		return errors.New("nothing received")
	}
	defer f.Close()
	if h.Size > a.opts.MaxFileSize {
		return errors.New("maximum file size is 100KB")
	}
	slurp, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%x", md5.Sum(slurp))
	_, err = present.Parse(bytes.NewReader(slurp), key, 0)
	if err != nil {
		// not valid or bad file
		return err
	}
	if err := a.saveSlide(key, slurp); err != nil {
		// even if it exists, we write again to update ModTime
		return err
	}
	return a.serveTemplate(w, "upload", struct {
		URL   string
		Hours string
	}{
		URL:   fmt.Sprintf("https://gopresent.io/slide/%s", key),
		Hours: strconv.Itoa(a.opts.MaxHours),
	})
}

// saveSlide writes the slide with the given key, containing the given data to disk.
func (a *app) saveSlide(key string, data []byte) error {
	slidePath := filepath.Join(a.opts.StorageRoot, "slides")
	fi, err := os.Stat(slidePath)
	if err != nil {
		return err
	}
	if fi.Size() > a.opts.MaxQuota {
		log.Printf("disk quota exceeded: %d > %d\n", fi.Size(), int(a.opts.MaxQuota))
		return errors.New("disk quota exceeded")
	}
	return ioutil.WriteFile(filepath.Join(slidePath, key), data, os.ModePerm)
}
