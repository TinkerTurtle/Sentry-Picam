package raspivid

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"simple-webcam/broker"
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

	return fmt.Sprintf(fileFormat+"_%04d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0), 0
}

func (rec *Recorder) checkFfmpeg() {
	_, err := exec.LookPath("ffmpeg")
	if err == nil {
		rec.hasFfmpeg = true
	}
}

// Init initializes the raspivid recorder. folderpath must include the trailing slash
// When recording is triggered by (rec.StoptTime > now), up to numHeaders Iframes will be
// saved before the trigger
func (rec *Recorder) Init(caster *broker.Broker, folderpath string) {
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
				if startedFile == false {
					fileName, fileCounter = getFilename(fileName, fileCounter)
					f, _ = os.Create(folderpath + fileName + extension)
					defer f.Close()
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
			} else if startedFile == true {
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
