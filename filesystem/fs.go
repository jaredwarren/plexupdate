package filesystem

import (
	"io/ioutil"
	"os"
	"strings"
)

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
	return err == nil || os.IsNotExist(err)
}
