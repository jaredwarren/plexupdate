package app

import (
	"mime"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/plexupdate/config"
)

// Service ...
type Service struct {
	Name   string
	Mux    *mux.Router
	Config config.Configuration
	Exit   chan error
}

// New instantiates a service with the given name.
func New(name string, conf config.Configuration) *Service {
	mux := mux.NewRouter()

	mux.HandleFunc("/static/{filename:[a-zA-Z0-9\\.\\-\\_\\/]*}", FileServer)

	var service = &Service{
		Name:   name,
		Mux:    mux,
		Config: conf,
		Exit:   make(chan error),
	}

	return service
}

// FileServer serves a file with mime type header
func FileServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	file := vars["filename"]
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(file)))
	http.ServeFile(w, r, "./static/"+file)
}
