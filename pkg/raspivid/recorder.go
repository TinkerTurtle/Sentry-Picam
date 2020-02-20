package raspivid

import (
	"fmt"
	"log"
	"os"
	"simple-webcam/broker"
	"time"
)

// Recorder writes the video stream to disk
type Recorder struct {
	RequestedRecord bool
	StopTime        time.Time
}

func getFilename(index int) string {
	fileFormat := "%d-%02d-%02d-%02d00_%04d"
	now := time.Now()
	return fmt.Sprintf(fileFormat, now.Year(), now.Month(), now.Day(), now.Hour(), index)
}

// Init initializes the raspivid recorder. folderpath must include the trailing slash
// When recording is triggered by (rec.StoptTime > now), up to numHeaders Iframes will be
// saved before the trigger
func (rec *Recorder) Init(caster *broker.Broker, folderpath string) {
	extension := ".h264"
	stream := caster.Subscribe()
	numHeaders := 0
	fileCounter := 0

	fileName := getFilename(fileCounter)

	f, _ := os.Create(folderpath + fileName + extension)
	defer f.Close()

	buf := [][]byte{}
	i := 0
	startedFile := false
	for {
		x := <-stream

		if rec.RequestedRecord {
			if time.Now().Before(rec.StopTime) {
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

				fileCounter++
				fileName = getFilename(fileCounter)
				f.Close()
				f, _ = os.Create(folderpath + fileName + extension)
				defer f.Close()
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
