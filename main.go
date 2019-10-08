package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/jaredwarren/plexupdate/app"
	"github.com/jaredwarren/plexupdate/command"
	"github.com/jaredwarren/plexupdate/config"
	"github.com/jaredwarren/plexupdate/form"
	"github.com/jaredwarren/plexupdate/youtube"
	"github.com/spf13/viper"
)

var conf config.Configuration

func main() {
	// config
	viper.SetConfigName("config_" + runtime.GOOS)
	viper.AddConfigPath(".")
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
	err := viper.Unmarshal(&conf)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	// Reload config on change
	viper.OnConfigChange(func(e fsnotify.Event) {
		// reset config, so deleted values go away
		conf = config.Configuration{}
		// reload new config
		err := viper.Unmarshal(&conf)
		if err != nil {
			log.Fatalf("unable to decode into struct, %v", err)
		}
	})

	service := app.New("Plex", conf)

	// Static file handler
	// service.Mux.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	service.Mux.HandleFunc("/", Home).Methods("GET") // for now just go to update.
	service.Mux.HandleFunc("/upload", Upload).Methods("GET")
	service.Mux.HandleFunc("/upload", UploadHandler).Methods("POST")

	// ytdl
	youtube.Register(service)

	// command
	command.Register(service)

	exit := make(chan error)

	// Interrupt handler (ctrl-c)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		done := <-signalChan
		fmt.Println("\nReceived an interrupt, stopping services...\n")
		// TODO: cleanup here.....
		exit <- fmt.Errorf("%s", done)
	}()

	// Start Server
	srv := &http.Server{
		Addr:    ":8081",
		Handler: service.Mux,
	}
	go func() {
		fmt.Printf("HTTP server listening on %q\n", srv.Addr)
		exit <- srv.ListenAndServe()
	}()

	// Wait for exit signal.
	fmt.Printf("\nexiting (%v)\n", <-exit)

	command.Cleanup()

	fmt.Println("Good Bye!")
}

// Home ...
func Home(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Home", r.URL.String())

	// parse every time to make updates easier, and save memory
	tpl := template.Must(template.New("base").ParseFiles("templates/home.html", "templates/base.html"))
	tpl.ExecuteTemplate(w, "base", &struct {
		Title string
	}{
		Title: "Home",
	})
}

// Upload ...
func Upload(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Update", r.URL.String())

	// parse every time to make updates easier, and save memory
	tpl := template.Must(template.New("base").Funcs(template.FuncMap{"CsrfToken": form.CsrfToken}).ParseFiles("templates/upload.html", "templates/base.html"))
	tpl.ExecuteTemplate(w, "base", &struct {
		Title     string
		Locations map[string]string
	}{
		Title:     "User List",
		Locations: conf.Plex.Locations,
	})
}

// UploadHandler ...
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	fmt.Println("UploadHandler", r.URL.String())

	// 3200 MB files max.
	r.Body = http.MaxBytesReader(w, r.Body, 3200<<20)
	if err := r.ParseMultipartForm(3200 << 20); err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}

	// setup root dir
	location := r.PostForm.Get("location")
	rootDir := conf.Plex.Locations[location]
	if rootDir == "" {
		rootDir = "./uploads"
	}
	err = os.MkdirAll(rootDir, os.ModePerm)
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}

	// get form file
	file, handler, err := r.FormFile("video_file")
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}
	defer file.Close()

	// TODO: get dir
	f, err := os.OpenFile(filepath.Join(rootDir, handler.Filename), os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}
	defer f.Close()
	_, err = io.Copy(f, file)
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}

	fmt.Println("  DONE!")
	w.Write([]byte("DONE"))
}

func internalError(ws *websocket.Conn, msg string, err error) {
	fmt.Println(msg, err)
	ws.WriteMessage(websocket.TextMessage, []byte("Internal server error."))
}
