package gopresent

import (
	"bytes"
	"net/http"
)

// serveTemplate writes the template specifies by name to w using the given data. It returns an error
// if it fails. It does not write anything to w on error.
func (a *app) serveTemplate(w http.ResponseWriter, name string, data interface{}) error {
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
