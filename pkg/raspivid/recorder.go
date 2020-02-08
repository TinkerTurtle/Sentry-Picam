package raspivid

import (
	"os"
	"simple-webcam/broker"
	"time"
)

// Recorder writes the video stream to disk
type Recorder struct {
	RequestedRecord bool
	StopTime        time.Time
}

// Init initializes the raspivid recorder
func (rec *Recorder) Init(caster *broker.Broker, filepath string) {
	stream := caster.Subscribe()
	numHeaders := 0

	f, _ := os.Create(filepath)
	defer f.Close()

	buf := [][]byte{}
	i := 0
	for {
		x := <-stream

		if rec.RequestedRecord && time.Now().Before(rec.StopTime) {
			for _, v := range buf {
				f.Write(v)
			}
			buf = buf[:0]
			numHeaders = 0
			i = 0
			f.Write(x.([]byte))
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
