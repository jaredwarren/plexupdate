package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/jaredwarren/plexupdate/command"
	"github.com/jaredwarren/plexupdate/config"
	"github.com/jaredwarren/plexupdate/filesystem"
	"github.com/jaredwarren/plexupdate/form"
	"github.com/rylio/ytdl"
	"github.com/spf13/viper"
)

var conf config.Configuration
var runningCommands map[string]*command.Command

func main() {
	runningCommands = make(map[string]*command.Command)

	// config

	// TODO: config name darwin, win, linux .....
	viper.SetConfigName("config")
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

	mux := mux.NewRouter()

	// Static file handler
	mux.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	mux.HandleFunc("/", Home).Methods("GET") // for now just go to update.
	mux.HandleFunc("/upload", Upload).Methods("GET")
	mux.HandleFunc("/upload", UploadHandler).Methods("POST")

	mux.HandleFunc("/youtube", Ytdl).Methods("GET")
	mux.HandleFunc("/ytdl", YtdlHandler).Methods("POST")

	// command
	mux.HandleFunc("/cmd", CommandList).Methods("GET")
	mux.HandleFunc("/cmd/{id}", Command).Methods("GET")
	mux.HandleFunc("/cmd/{id}", CommandHandler).Methods("POST")
	mux.HandleFunc("/cmd/ws/{id}", CmdWS).Methods("GET")

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

// CommandList ...
func CommandList(w http.ResponseWriter, r *http.Request) {
	fmt.Println("CommandList", r.URL.String())

	// TODO: get previous and current commands...
	files, err := ioutil.ReadDir("./logs")
	if err != nil {
		log.Fatal(err)
	}

	commands := []*command.Command{}
	for _, f := range files {
		fileName := f.Name()
		var ok bool
		cmd, ok := runningCommands[fileName]
		if !ok {
			cmd = command.LoadFile(fileName)
			cmd.Running = false
		}

		commands = append(commands, cmd)
		fmt.Println(f.Name())
	}

	// parse every time to make updates easier, and save memory
	tpl := template.Must(template.New("base").ParseFiles("templates/cmd/command_list.html", "templates/base.html"))
	tpl.ExecuteTemplate(w, "base", &struct {
		Title    string
		Commands []*command.Command
	}{
		Title:    "Home",
		Commands: commands,
	})
}

// Command shows start command form for "new" commands, and logs for old/done
func Command(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Command", r.URL.String())

	vars := mux.Vars(r)
	cmdID := vars["id"]

	if cmdID == "" {
		w.Write([]byte("ID Missing:" + cmdID))
		return
	}

	if cmdID == "new" {
		// parse every time to make updates easier, and save memory
		tpl := template.Must(template.New("base").ParseFiles("templates/cmd/command.html", "templates/base.html"))
		tpl.ExecuteTemplate(w, "base", &struct {
			Title string
		}{
			Title: "Home",
		})
	} else {
		var cmd *command.Command
		var ok bool
		cmd, ok = runningCommands[cmdID]
		if !ok {
			filePath := fmt.Sprintf("./logs/%s.out", cmdID)
			if filesystem.Exists(filePath) {
				cmd = &command.Command{
					ID:      cmdID,
					Running: false,
					LogFile: filePath,
				}
			} else {
				w.Write([]byte("Log Missing:" + cmdID))
				return
			}
		}

		if cmd == nil {
			w.Write([]byte("cmd Missing:" + cmdID))
			return
		}

		// load file contents first
		fileData, _ := ioutil.ReadFile(cmd.LogFile)

		// parse every time to make updates easier, and save memory
		tpl := template.Must(template.New("base").ParseFiles("templates/cmd/logs.html", "templates/base.html"))
		tpl.ExecuteTemplate(w, "base", &struct {
			Title    string
			Cmd      *command.Command
			FileData string
		}{
			Title:    "Home",
			Cmd:      cmd,
			FileData: string(fileData),
		})
	}
}

// CommandHandler ...
func CommandHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("CommandHandler", r.URL.String())

	vars := mux.Vars(r)
	cmdID := vars["id"]
	if cmdID != "" {
		cmdID = "new"
	}

	if cmdID != "new" {
		http.Redirect(w, r, "/cmd/"+cmdID, http.StatusSeeOther)
		return
	}

	r.ParseForm()

	c := r.FormValue("cmd")
	d := r.FormValue("dir")
	fmt.Println(">>>", d, c)

	// TODO: check if command is being run already?

	// store for later, so ws can find it
	cmd := command.NewCommand("ping 192.168.0.111")
	runningCommands[cmd.ID] = cmd

	// start command now, so
	go cmd.Start()

	http.Redirect(w, r, "/cmd/"+cmd.ID, http.StatusSeeOther)
}

// TODO: make page to list commands, running and finished
// TODO: make handler to show command output, running and finished, whthout running again...
//   - rename file to something like .done.out, when done?
// TODO: fix ws to read the file
//  - https://stackoverflow.com/questions/10135738/reading-log-files-as-theyre-updated-in-go

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// CmdWS ...
func CmdWS(w http.ResponseWriter, r *http.Request) {
	fmt.Println("TestWS", r.URL.String())
	vars := mux.Vars(r)
	cmdID := vars["id"]

	cmd, ok := runningCommands[cmdID]
	if !ok {
		w.Write([]byte("Cmd not found:" + cmdID))
	}

	// Web socket
	var upgrader = websocket.Upgrader{}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer ws.Close()

	done := make(chan bool)
	go ping(ws, done)
	go pumpStdIn(ws, cmd)

	pumpStdOut(ws, cmd, done)

	//
	// old
	//

	// roomID := r.URL.Query().Get("room")
	// fmt.Println("  roomID:", roomID)

	// cmd, ok := rooms[roomID]
	// if !ok {
	// 	panic("room cmd missing::" + roomID)
	// }

	// stdoutDone := make(chan bool)
	// go pumpStdOut(ws, outr, stdoutDone)

	// pumpStdIn(ws, proc)

	// // Some commands will exit when stdin is closed.
	// ws.Close()
	// inw.Close()

	// // Other commands need a bonk on the head.
	// if err := proc.Signal(os.Interrupt); err != nil {
	// 	fmt.Println("inter:", err)
	// }

	// select {
	// case <-stdoutDone:
	// case <-time.After(time.Second):
	// 	// A bigger bonk on the head.
	// 	if err := proc.Signal(os.Kill); err != nil {
	// 		fmt.Println("term:", err)
	// 	}
	// 	<-stdoutDone
	// }

	// if _, err := proc.Wait(); err != nil {
	// 	fmt.Println("wait:", err)
	// }
}

func pumpStdIn(ws *websocket.Conn, cmd *command.Command) {
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			fmt.Println("ReadMessage:", err)
			break
		}
		action := string(message)
		fmt.Println(" <<<<< ", action)
		if action == "kill" {
			cmd.Cmd.Process.Kill()
			return
		}
	}
}

func pumpStdOut(ws *websocket.Conn, cmd *command.Command, done chan bool) {
	// TODO: move most of this to filesystem.Watch, return io.Reader compatable struct
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(cmd.LogFile)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Open(cmd.LogFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// get the size
	fi, err := file.Stat()
	if err != nil {
		log.Fatal(err)
	}
	oldSize := fi.Size()

	// // write whole file to ws
	// fb := make([]byte, oldSize)
	// _, err = file.Read(fb)
	// if err := ws.WriteMessage(websocket.TextMessage, fb); err != nil {
	// 	fmt.Printf("%+v\n", err)
	// }

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				ws.SetWriteDeadline(time.Now().Add(writeWait))

				// get last x bytes of file and write to web socket
				fi, err := file.Stat()
				if err != nil {
					log.Fatal(err)
				}
				size := fi.Size()
				sizeDif := (size - oldSize)
				if sizeDif > 0 {
					buf := make([]byte, sizeDif)
					start := size - sizeDif
					_, err = file.ReadAt(buf, start)
					if err == nil {
						// fmt.Printf("  - %s\n", buf)
						if err := ws.WriteMessage(websocket.TextMessage, buf); err != nil {
							fmt.Println("WriteMessage:", err)
							ws.Close()
							break
						}
					}

					// reset old size
					oldSize = size
				}
			}
		case <-done:
			return
		}
	}

	// close(done)

	// ws.SetWriteDeadline(time.Now().Add(writeWait))
	// ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	// time.Sleep(closeGracePeriod)
}

// I think this just keeps the websocket connection alive
func ping(ws *websocket.Conn, done chan bool) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				fmt.Println("ping:", err)
			}
		case <-done:
			return
		}
	}
}

//
//
//
//
//
//

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

/**
* YTDL
 */

// Ytdl ...
func Ytdl(w http.ResponseWriter, r *http.Request) {
	// parse every time to make updates easier, and save memory
	templates := template.Must(template.ParseFiles("templates/ytdl.html", "templates/base.html"))
	templates.ExecuteTemplate(w, "base", &struct {
		Title     string
		Locations map[string]string
	}{
		Title:     "YTDL",
		Locations: conf.Plex.Locations,
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

	// setup root dir
	location := r.PostForm.Get("location")
	rootDir := conf.Plex.Locations[location]
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

func downloadVideo(id, rootDir string, audioOnly bool) (string, error) {
	os.MkdirAll(rootDir, os.ModePerm)

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
	fileName := filepath.Join(rootDir, filesystem.SanitizeFilename(vid.Title, false)+"."+format.Extension)
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

//
//
//
// CsrfToken returns token
func CsrfToken() string {
	return form.New()
}

func internalError(ws *websocket.Conn, msg string, err error) {
	fmt.Println(msg, err)
	ws.WriteMessage(websocket.TextMessage, []byte("Internal server error."))
}
