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

func getFilename() string {
	fileFormat := "%d-%02d-%02d-%02d00.h264"
	now := time.Now()
	return fmt.Sprintf(fileFormat, now.Year(), now.Month(), now.Day(), now.Hour())
}

// Init initializes the raspivid recorder. folderpath must include the trailing slash
func (rec *Recorder) Init(caster *broker.Broker, folderpath string) {
	stream := caster.Subscribe()
	numHeaders := 0

	fileName := getFilename()

	f, _ := os.Create(folderpath + fileName)
	defer f.Close()

	buf := [][]byte{}
	i := 0
	for {
		x := <-stream

		if rec.RequestedRecord && time.Now().Before(rec.StopTime) {
			newName := getFilename()
			if fileName != newName {
				fileName = newName
				f.Close()
				f, _ = os.Create(folderpath + fileName)
				defer f.Close()
			}

			for _, v := range buf {
				_, err := f.Write(v)
				if err != nil {
					log.Println(err)
				}
			}
			buf = buf[:0]
			numHeaders = 0
			i = 0
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
