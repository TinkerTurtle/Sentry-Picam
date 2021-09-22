package raspivid

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sentry-picam/broker"
	"time"
)

// Recorder writes the video stream to disk
type Recorder struct {
	RequestedRecord bool
	StopTime        time.Time
	hasFfmpeg       bool
}

func getFilename(lastName string, counter int) (string, int) {
	fileFormat := "%d-%02d-%02d-%02d%02d"
	now := time.Now()
	newFilename := fmt.Sprintf(fileFormat, now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute())
	if newFilename == lastName {
		counter++
		return newFilename + fmt.Sprintf("_%02d", counter), counter
	}

	return fmt.Sprintf(fileFormat+"_%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0), 0
}

func (rec *Recorder) checkFfmpeg() bool {
	_, err := exec.LookPath("ffmpeg")
	if err == nil {
		rec.hasFfmpeg = true
	}

	return rec.hasFfmpeg
}

// Init initializes the raspivid recorder. folderpath must include the trailing slash
// When recording is triggered by (rec.StopTime > now), up to numHeaders Iframes will be
// saved before the trigger
func (rec *Recorder) Init(caster *broker.Broker, folderpath string, framerate int, triggerScript string) {
	os.MkdirAll(folderpath+"raw/", 0777)

	converter := Converter{}
	converter.Framerate = framerate
	converter.TriggerScript = triggerScript
	if rec.checkFfmpeg() {
		go converter.Start(rec, folderpath)
	}

	extension := ".h264"
	stream := caster.Subscribe()
	defer caster.Unsubscribe(stream)
	numHeaders := 0
	fileCounter := 0

	var f *os.File
	var fileName string

	buf := [][]byte{}
	i := 0
	startedFile := false
	for {
		x := <-stream

		if rec.RequestedRecord {
			if time.Now().Before(rec.StopTime) {
				if !startedFile {
					fileName, fileCounter = getFilename(fileName, fileCounter)
					f, _ = os.Create(folderpath + "raw/" + fileName + extension)
				}

				startedFile = true

				for _, v := range buf {
					_, err := f.Write(v)
					if err != nil {
						log.Println(err)
					}
				}
				buf = buf[:0]
				numHeaders = 0
				i = 0
			} else if startedFile {
				startedFile = false

				f.Close()
			}
		}

		if x.([]byte)[4] == 39 { // always start with SPS header
			if numHeaders == 2 {
				buf = buf[i:]
				numHeaders = 0
				i = 0
			}
			numHeaders++
		}

		buf = append(buf, x.([]byte))
		i++
	}
}
