package youtube

import (
	"fmt"
	"html/template"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/plexupdate/app"
	"github.com/jaredwarren/plexupdate/config"
)

// Controller implements the home resource.
type Controller struct {
	mux  *mux.Router
	conf config.Configuration
}

// Register ...
func Register(service *app.Service) {
	uc := &Controller{
		mux:  service.Mux,
		conf: service.Config,
	}
	uc.MountController()
}

// MountController ...
func (c *Controller) MountController() {
	c.mux.HandleFunc("/youtube", c.Ytdl).Methods("GET")
	c.mux.HandleFunc("/ytdl", c.YtdlHandler).Methods("POST")
}

// Ytdl ...
func (c *Controller) Ytdl(w http.ResponseWriter, r *http.Request) {
	// parse every time to make updates easier, and save memory
	templates := template.Must(template.ParseFiles("templates/ytdl.html", "templates/base.html"))
	templates.ExecuteTemplate(w, "base", &struct {
		Title     string
		Locations map[string]string
	}{
		Title:     "YTDL",
		Locations: c.conf.Plex.Locations,
	})
}

// YtdlHandler ...
func (c *Controller) YtdlHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("YtdlHandler:", r.URL.String())

	r.ParseForm()
	id := r.FormValue("id")
	if id == "" {
		fmt.Println("  Empty username")
		w.Write([]byte(" Empty username"))
		return
	}

	audioOnly := r.FormValue("audio") == "on"

	// setup root dir
	location := r.PostForm.Get("location")
	rootDir := c.conf.Plex.Locations[location]
	if rootDir == "" {
		rootDir = "./uploads"
	}
	err := os.MkdirAll(rootDir, os.ModePerm)
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}

	// TODO: get dir...
	fileName, err := downloadVideo(id, rootDir, audioOnly)
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}

	w.Write([]byte("Success:" + fileName))
}
