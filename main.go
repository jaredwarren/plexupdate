package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/jaredwarren/rpi_pic/user"
)

func main() {

	mux := mux.NewRouter()

	// Static file handler
	mux.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	mux.HandleFunc("/root/update", Update).Methods("GET")
	mux.HandleFunc("/root/update", UpdateHandler).Methods("POST")

}

// Update ...
func Update(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Update", r.URL.String())
	// session, _ := c.cookieStore.Get(r, "user-session")
	// Check for duplicate username
	// messages := user.GetMessages(session)

	// parse every time to make updates easier, and save memory
	tpl := template.Must(template.New("base").ParseFiles("templates/upload.html", "templates/base.html"))
	tpl.ExecuteTemplate(w, "base", &struct {
		Title string
		// Messages []string
	}{
		Title: "User List",
		// Messages: messages,
	})
	// session.Save(r, w)
}

// UpdateHandler ...
func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("UpdateHandler", r.URL.String())

	// get session
	// session, _ := c.cookieStore.Get(r, "user-session")

	// 32 MB files max.
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// session.AddFlash(err.Error())
		// session.Save(r, w)
		http.Redirect(w, r, "/root/update", http.StatusFound)
		return
	}

	// // make sure user pic dir exists
	backupPath := "backup/"
	os.RemoveAll(backupPath)
	os.MkdirAll(backupPath, os.ModePerm)

	// get form file
	var err error
	file, _, err := r.FormFile("package")
	defer file.Close()
	if err != nil {
		fmt.Println("  [E] form file")
		// session.AddFlash(err.Error())
		// session.Save(r, w)
		http.Redirect(w, r, "/root/update", http.StatusFound)
		return
	}

	// unzip file
	zr, err := zip.NewReader(file, r.ContentLength)
	if err != nil {
		fmt.Println("  [E] zip read:", err)
		// session.AddFlash(err.Error())
		// session.Save(r, w)
		http.Redirect(w, r, "/root/update", http.StatusFound)
		return
	}

	// process each file
	err = nil
	for _, zf := range zr.File {
		fmt.Println("  <-- ", zf.Name)
		// Ignore garbage files
		if strings.HasPrefix(zf.Name, "__") {
			continue
		}
		// Ignore hidden files
		if strings.HasPrefix(zf.Name, ".") {
			continue
		}

		var src io.ReadCloser
		src, err = zf.Open()
		if err != nil {
			fmt.Println("  [E] zf.Open:", err)
			break // ?
		}

		path := zf.Name
		if zf.FileInfo().IsDir() {
			os.MkdirAll(path, os.ModePerm)
		} else {
			// check current file
			_, err = os.Stat(path)
			if os.IsNotExist(err) {
				// file doesn't exists, that's ok
			} else if err != nil {
				// some other error
				fmt.Println("  [E] stat:", err)
				break // ?
			} else {
				os.MkdirAll(filepath.Dir(backupPath+path), os.ModePerm)
				// file exists, back up old file
				fmt.Println("  > backup:", path, backupPath+path)
				err = os.Rename(path, backupPath+path)
				if err != nil {
					fmt.Println("  [E] copy backup:", err)
					break // ?
				}
			}

			// create file dir
			os.MkdirAll(filepath.Dir(path), os.ModePerm)

			// copy file
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			if err != nil {
				fmt.Println("  [E] zf.Open:", err)
				break // ?
			}

			_, err = io.Copy(f, src)
			if err != nil {
				fmt.Println("  [E] io.Copy:", err)
				break // ?
			}

			// close now, don't `defer`
			f.Close()
			src.Close()
		}
	}

	// roll back files if anything goes wrong
	if err != nil {
		// basically same as "Restore"
		err2 := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
			if info == nil {
				return nil
			}
			if !info.IsDir() {
				newPath := strings.TrimPrefix(path, backupPath)
				fmt.Println("  > restore:", path, newPath)
				return CopyFile(path, newPath)
			}
			return nil
		})
		if err2 != nil {
			fmt.Println("  err2:", err2)
		}

		// display error
		// session.AddFlash(err.Error())
		// session.Save(r, w)
		http.Redirect(w, r, "/root/update", http.StatusFound)
		return
	}

	fmt.Println("  DONE!")

	// redirect back to picture list
	http.Redirect(w, r, "/root/restart?csrf_token="+user.CsrfToken(), http.StatusFound)
}

// IsDirEmpty ...
func IsDirEmpty(name string) bool {
	files, _ := ioutil.ReadDir(name)
	return len(files) > 0
}

// CopyFile ...
func CopyFile(source, dest string) error {
	if exists := Exists(source); !exists {
		return nil
	}
	input, err := ioutil.ReadFile(source)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dest, input, 0644)
	if err != nil {
		return err
	}
	return nil
}

// Exists does file or directory exists?
func Exists(filename string) bool {
	_, err := os.Stat(filename)
	return os.IsNotExist(err)
}
