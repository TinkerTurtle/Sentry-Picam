package raspivid

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Converter struct {
	Framerate      int
	TriggerScript  string
	highlightCache map[string]float64
	recorder       *Recorder
	folder         string
}

func (conv *Converter) CacheItem(filename string, time float64) {
	conv.highlightCache[filename] = time
}

func (conv *Converter) convertFile(name string) {
	s := strings.Split(name, "-")
	newFolder := fmt.Sprintf("%s/%s-%s/", conv.folder, s[0], s[1])
	os.MkdirAll(newFolder, 0777)

	cmd := exec.Command("nice", "-19",
		"ffmpeg", "-y",
		"-framerate", strconv.Itoa(conv.Framerate),
		"-i", conv.folder+"raw/"+name+".h264",
		"-c", "copy",
		newFolder+name+".mp4",
	)
	cmd.Run()

	skip, ok := conv.highlightCache[name]
	if !ok {
		skip = 3
	}
	cmd = exec.Command("nice", "-19",
		"ffmpeg", "-y",
		"-ss", fmt.Sprintf("%f", skip),
		"-i", newFolder+name+".mp4",
		"-vf", "scale=600:-1",
		"-qscale:v", "16",
		"-frames:v", "1",
		newFolder+name+".jpg",
	)
	cmd.Run()
	delete(conv.highlightCache, name)

	os.Remove(conv.folder + "raw/" + name + ".h264")
	log.Println("File written: ", name, "Offset:", skip)

	if conv.TriggerScript != "" {
		cmd = exec.Command("nice", "-19",
			conv.TriggerScript, name,
		)
		err := cmd.Start()
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (conv *Converter) convertFolder(folder string) {
	files, err := os.ReadDir(folder + "raw/")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		extension := filepath.Ext(strings.ToLower(f.Name()))
		name := strings.TrimSuffix(filepath.Base(f.Name()), filepath.Ext(f.Name()))

		if extension == ".h264" {
			conv.convertFile(name)
		}
	}
}

func (conv *Converter) Init(rec *Recorder, folder string) {
	conv.recorder = rec
	conv.folder = folder
	conv.highlightCache = make(map[string]float64)
}

func (conv *Converter) Start(rec *Recorder, folder string) {
	for {
		if time.Now().After(rec.StopTime) {
			conv.convertFolder(folder)
		}
		time.Sleep(5 * time.Second)
	}
}
