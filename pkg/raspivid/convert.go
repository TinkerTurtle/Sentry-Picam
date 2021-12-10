package raspivid

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Converter struct {
	Framerate     int
	TriggerScript string
}

func (conv *Converter) convert(folder string) {
	files, err := os.ReadDir(folder + "raw/")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		extension := filepath.Ext(strings.ToLower(f.Name()))
		name := strings.TrimSuffix(filepath.Base(f.Name()), filepath.Ext(f.Name()))

		if extension == ".h264" {
			cmd := exec.Command("nice", "-19",
				"ffmpeg", "-y",
				"-framerate", strconv.Itoa(conv.Framerate),
				"-i", folder+"raw/"+f.Name(),
				"-c", "copy",
				folder+name+".mp4",
			)
			cmd.Run()

			cmd = exec.Command("nice", "-19",
				"ffmpeg", "-y",
				"-ss", "3",
				"-i", folder+name+".mp4",
				"-vf", "scale=600:-1",
				"-qscale:v", "16",
				"-frames:v", "1",
				folder+name+".jpg",
			)
			cmd.Run()

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

func (conv *Converter) Start(rec *Recorder, folder string) {
	for {
		if time.Now().After(rec.StopTime) {
			conv.convert(folder)
		}
		time.Sleep(5 * time.Second)
	}
}
