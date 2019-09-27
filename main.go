package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/plexupdate/config"
	"github.com/jaredwarren/plexupdate/form"
	"github.com/rylio/ytdl"
	"github.com/spf13/viper"
)

var conf config.Configuration

func main() {
	// config
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
	err := viper.Unmarshal(&conf)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	mux := mux.NewRouter()

	// Static file handler
	mux.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	mux.HandleFunc("/", Home).Methods("GET") // for now just go to update.
	mux.HandleFunc("/upload", Upload).Methods("GET")
	mux.HandleFunc("/upload", UploadHandler).Methods("POST")

	mux.HandleFunc("/youtube", Ytdl).Methods("GET")
	mux.HandleFunc("/ytdl", YtdlHandler).Methods("POST")

	exit := make(chan error)

	// Interrupt handler
	go func() {
		c := make(chan os.Signal, 1)
		exit <- fmt.Errorf("%s", <-c)
	}()

	// Start Server
	srv := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}
	go func() {
		fmt.Printf("HTTP server listening on %q\n", srv.Addr)
		exit <- srv.ListenAndServe()
	}()

	// Wait for signal.
	fmt.Printf("\nexiting (%v)\n", <-exit)
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
	tpl := template.Must(template.New("base").Funcs(template.FuncMap{"CsrfToken": CsrfToken}).ParseFiles("templates/upload.html", "templates/base.html"))
	tpl.ExecuteTemplate(w, "base", &struct {
		Title string
	}{
		Title: "User List",
	})
}

// UploadHandler ...
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("UploadHandler", r.URL.String())

	// 3200 MB files max.
	r.Body = http.MaxBytesReader(w, r.Body, 3200<<20)
	if err := r.ParseMultipartForm(3200 << 20); err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}

	fmt.Printf("MultipartForm:%+v", r.MultipartForm)
	fmt.Printf("PostForm:%+v", r.PostForm)

	location := r.PostForm.Get("location")

	fmt.Printf("%+v\n", location)

	rootDir := conf.Plex.Locations[location]
	if rootDir != "" {
		fmt.Printf("rootDir:%+v\n", rootDir)
	} else {
		fmt.Println("????")
	}

	return

	// get form file
	var err error
	file, handler, err := r.FormFile("video_file")
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}
	defer file.Close()

	// TODO: get dir
	f, err := os.OpenFile("./"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}
	defer f.Close()
	io.Copy(f, file)

	fmt.Println("  DONE!")
	w.Write([]byte("DONE"))
}

/**
* YTDL
 */

// Ytdl ...
func Ytdl(w http.ResponseWriter, r *http.Request) {
	// parse every time to make updates easier, and save memory
	templates := template.Must(template.ParseFiles("templates/ytdl.html", "templates/base.html"))
	templates.ExecuteTemplate(w, "base", &struct {
		Title string
	}{
		Title: "YTDL",
	})
}

// YtdlHandler ...
func YtdlHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("YtdlHandler:", r.URL.String())

	r.ParseForm()
	id := r.FormValue("id")
	if id == "" {
		fmt.Println("  Empty username")
		w.Write([]byte(" Empty username"))
		return
	}

	audioOnly := r.FormValue("audio") == "on"

	// TODO: get dir...
	fileName, err := downloadVideo(id, "./", audioOnly)
	if err != nil {
		w.Write([]byte(" [E]:" + err.Error()))
		return
	}

	w.Write([]byte("Success:" + fileName))
}

func downloadVideo(id, rootPath string, audioOnly bool) (string, error) {
	os.MkdirAll(rootPath, os.ModePerm)

	vid, err := ytdl.GetVideoInfo(id)
	if err != nil {
		fmt.Println("  ", err)
		return "", err
	}
	var format ytdl.Format
	if audioOnly {
		format = vid.Formats.Best("audbr")[0]
	} else {
		format = vid.Formats.Best("videnc")[0]
	}
	fileName := rootPath + SanitizeFilename(vid.Title, false) + "." + format.Extension
	file, err := os.Create(fileName)
	defer file.Close()
	if err != nil {
		fmt.Println("  ", err)
		return fileName, err
	}
	err = vid.Download(vid.Formats[0], file)
	if err != nil {
		fmt.Println("  ", err)
		return fileName, err
	}

	// Convert to mp3
	if audioOnly {
		videoFile := fileName
		fileName, err = convertVideoToMP3(fileName)
		if err != nil {
			fmt.Println("  ", err)
			return fileName, err
		}
		// cleanup video file
		os.Remove(videoFile)
	}
	return fileName, nil
}

func convertVideoToMP3(videoPath string) (string, error) {
	// ffmpeg -i video.mp4 -q:a 0 -map a audio.mp3
	destName := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	audioPath := destName + ".mp3"
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-q:a", "0", "-map", "a", audioPath)
	err := cmd.Run()
	return audioPath, err
}

var badCharacters = []string{
	"../",
	"<!--",
	"-->",
	"<",
	">",
	"'",
	"\"",
	"&",
	"$",
	"#",
	"{", "}", "[", "]", "=",
	";", "?", "%20", "%22",
	"%3c",   // <
	"%253c", // <
	"%3e",   // >
	"",      // > -- fill in with % 0 e - without spaces in between
	"%28",   // (
	"%29",   // )
	"%2528", // (
	"%26",   // &
	"%24",   // $
	"%3f",   // ?
	"%3b",   // ;
	"%3d",   // =
}

// SanitizeFilename ...
func SanitizeFilename(name string, relativePath bool) string {
	if name == "" {
		return name
	}

	// if relativePath is TRUE, we preserve the path in the filename
	// If FALSE and will cause upper path foldername to merge with filename
	// USE WITH CARE!!!
	badDictionary := badCharacters
	if !relativePath {
		// add additional bad characters
		badDictionary = append(badCharacters, "./", "/")
	}

	// trim white space
	trimmed := strings.TrimSpace(name)

	// trim bad chars
	temp := trimmed
	for _, badChar := range badDictionary {
		temp = strings.Replace(temp, badChar, "", -1)
	}
	stripped := strings.Replace(temp, "\\", "", -1)
	return stripped
}

//
//
//
// CsrfToken returns token
func CsrfToken() string {
	return form.New()
}
