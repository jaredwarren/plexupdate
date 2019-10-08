package youtube

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jaredwarren/plexupdate/filesystem"
	"github.com/rylio/ytdl"
)

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
