package raspivid

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Converter struct {
	Framerate      int
	TriggerScript  string
	highlightCache map[string]float64
	Mutx           sync.Mutex
}

func (conv *Converter) CacheItem(filename string, time float64) {
	conv.highlightCache[filename] = time
}

func (conv *Converter) convert(folder string) {
	files, err := os.ReadDir(folder + "raw/")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		extension := filepath.Ext(strings.ToLower(f.Name()))
		name := strings.TrimSuffix(filepath.Base(f.Name()), filepath.Ext(f.Name()))
		s := strings.Split(name, "-")
		newFolder := fmt.Sprintf("%s/%s-%s/", folder, s[0], s[1])
		os.MkdirAll(newFolder, 0777)

		if extension == ".h264" {
			cmd := exec.Command("nice", "-19",
				"ffmpeg", "-y",
				"-framerate", strconv.Itoa(conv.Framerate),
				"-i", folder+"raw/"+f.Name(),
				"-c", "copy",
				newFolder+name+".mp4",
			)
			cmd.Run()

			conv.Mutx.Lock()
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
			conv.Mutx.Unlock()

			os.Remove(folder + "raw/" + f.Name())

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
	}
}

func (conv *Converter) Init() {
	conv.highlightCache = make(map[string]float64)
}

func (conv *Converter) Start(rec *Recorder, folder string) {
	for {
		if time.Now().After(rec.StopTime) {
			conv.convert(folder)
		}
		time.Sleep(5 * time.Second)
	}
}
