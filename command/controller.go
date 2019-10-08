package command

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/jaredwarren/plexupdate/app"
	"github.com/jaredwarren/plexupdate/filesystem"
)

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

// list if running commands
var runningCommands map[string]*Command

func init() {
	runningCommands = make(map[string]*Command)
}

// Controller implements the home resource.
type Controller struct {
	Mux *mux.Router
}

// Register ...
func Register(service *app.Service) {
	uc := &Controller{
		Mux: service.Mux,
	}
	uc.MountController()
}

// MountController ...
func (c *Controller) MountController() {
	c.Mux.HandleFunc("/cmd", c.CommandList).Methods("GET")
	c.Mux.HandleFunc("/cmd/{id}", c.Command).Methods("GET")
	c.Mux.HandleFunc("/cmd/{id}", c.CommandHandler).Methods("POST")
	c.Mux.HandleFunc("/cmd/ws/{id}", c.CmdWS).Methods("GET")
}

// CommandList ...
func (c *Controller) CommandList(w http.ResponseWriter, r *http.Request) {
	fmt.Println("CommandList", r.URL.String())

	// TODO: get previous and current commands...
	files, err := ioutil.ReadDir("./logs")
	if err != nil {
		log.Fatal(err)
	}

	commands := []*Command{}
	for _, f := range files {
		fileName := f.Name()
		var ok bool
		cmd, ok := runningCommands[fileName]
		if !ok {
			cmd = LoadFile(fileName)
		}

		commands = append(commands, cmd)
		fmt.Println(f.Name())
	}

	// parse every time to make updates easier, and save memory
	tpl := template.Must(template.New("base").ParseFiles("templates/cmd/command_list.html", "templates/base.html"))
	tpl.ExecuteTemplate(w, "base", &struct {
		Title    string
		Commands []*Command
	}{
		Title:    "Home",
		Commands: commands,
	})
}

// Command shows start command form for "new" commands, and logs for old/done
func (c *Controller) Command(w http.ResponseWriter, r *http.Request) {
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
		var cmd *Command
		var ok bool
		cmd, ok = runningCommands[cmdID]
		if !ok {
			filePath := fmt.Sprintf("./logs/%s.out", cmdID)
			if filesystem.Exists(filePath) {
				cmd = &Command{
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
			Cmd      *Command
			FileData string
		}{
			Title:    "Home",
			Cmd:      cmd,
			FileData: string(fileData),
		})
	}
}

// CommandHandler ...
func (c *Controller) CommandHandler(w http.ResponseWriter, r *http.Request) {
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

	cm := r.FormValue("cmd")
	d := r.FormValue("dir")
	fmt.Println(">>>", d, cm)

	// TODO: check if command is being run already?

	// store for later, so ws can find it
	cmd := NewCommand("ping 192.168.0.111")
	runningCommands[cmd.ID] = cmd

	// start command now, so
	go cmd.Start()

	http.Redirect(w, r, "/cmd/"+cmd.ID, http.StatusSeeOther)
}

// TODO: make page to list commands, running and finished
// TODO: make handler to show command output, running and finished, whthout running again...
// TODO: fix ws to read the file
//  - https://stackoverflow.com/questions/10135738/reading-log-files-as-theyre-updated-in-go

// CmdWS ...
func (c *Controller) CmdWS(w http.ResponseWriter, r *http.Request) {
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
}

func pumpStdIn(ws *websocket.Conn, cmd *Command) {
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
			cmd.Close()
			return
		}
	}
}

func pumpStdOut(ws *websocket.Conn, cmd *Command, done chan bool) {
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

// Cleanup kills all running commands
func Cleanup() {
	for _, cmd := range runningCommands {
		cmd.Close()
	}
}
