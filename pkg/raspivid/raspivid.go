package raspivid

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os/exec"
	"strconv"

	"simple-webcam/broker"
	h "simple-webcam/helper"
)

// Camera is a wrapper for raspivid
type Camera struct {
	Width, Height, Fps, Bitrate, SensorMode, Rotation *int
	CameraNightMode                                   chan bool
}

func (c *Camera) startDayCamera() (io.ReadCloser, *exec.Cmd) {
	cmd := exec.Command("raspivid",
		"-t", "0",
		"-o", "-",
		"-w", strconv.Itoa(*c.Width),
		"-h", strconv.Itoa(*c.Height),
		"-fps", strconv.Itoa(*c.Fps),
		"-rot", strconv.Itoa(*c.Rotation),
		"-mm", "backlit",
		"-drc", "high",
		"-b", strconv.Itoa(*c.Bitrate),
		"-md", strconv.Itoa(*c.SensorMode),
		"-pf", "baseline",
		"-g", strconv.Itoa(*c.Fps*2),
		"-ih", //"-stm",
		"-a", "1028",
		"-a", " %Y-%m-%d %l:%M:%S %P",
	)
	stdOut, err := cmd.StdoutPipe()
	h.CheckError(err)

	return stdOut, cmd
}

func (c *Camera) startNightCamera() (io.ReadCloser, *exec.Cmd) {
	cmd := exec.Command("raspivid",
		"-t", "0",
		"-o", "-",
		"-w", strconv.Itoa(*c.Width),
		"-h", strconv.Itoa(*c.Height),
		"-fps", "0",
		"-rot", strconv.Itoa(*c.Rotation),
		"-ex", "nightpreview",
		"-mm", "backlit",
		"-drc", "high",
		"-b", strconv.Itoa(*c.Bitrate),
		"-md", strconv.Itoa(*c.SensorMode),
		"-pf", "baseline",
		"-g", strconv.Itoa(*c.Fps*2),
		"-ih", //"-stm",
		"-a", "1028",
		"-a", " %Y-%m-%d %l:%M:%S %P",
	)
	stdOut, err := cmd.StdoutPipe()
	h.CheckError(err)

	return stdOut, cmd
}

// Start initializes the broadcast channel and starts raspivid
func (c *Camera) Start(caster *broker.Broker) {
	c.CameraNightMode = make(chan bool)

	if *c.Rotation == 90 || *c.Rotation == 270 {
		t := *c.Width
		*c.Width = *c.Height
		*c.Height = t
	}

	nalDelimiter := []byte{0, 0, 0, 1}
	searchLen := len(nalDelimiter)
	splitFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Return nothing if at end of file and no data passed
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		// Find the index of the NAL delimiter
		if i := bytes.Index(data, nalDelimiter); i >= 0 {
			return i + searchLen, data[0:i], nil
		}

		// If at end of file with data return the data
		if atEOF {
			return len(data), data, nil
		}

		return
	}

	stdOut, cmd := c.startDayCamera()
	log.Println("Camera Online")

	buffer := make([]byte, *c.Bitrate/5)
	s := bufio.NewScanner(io.Reader(stdOut))
	s.Buffer(buffer, len(buffer))
	s.Split(splitFunc)
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case nightMode := <-c.CameraNightMode:
			if nightMode {
				log.Println("Switching to night mode")
				err := cmd.Process.Kill()
				h.CheckError(err)

				stdOut, cmd = c.startNightCamera()
				s = bufio.NewScanner(io.Reader(stdOut))
				s.Buffer(buffer, len(buffer))
				s.Split(splitFunc)
				if err := cmd.Start(); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Println("Switching to day mode")
				err := cmd.Process.Kill()
				h.CheckError(err)

				stdOut, cmd = c.startDayCamera()
				s = bufio.NewScanner(io.Reader(stdOut))
				s.Buffer(buffer, len(buffer))
				s.Split(splitFunc)
				if err := cmd.Start(); err != nil {
					log.Fatal(err)
				}
			}
		default:
			if s.Scan() == false {
				log.Fatal("Bitrate should be increased to workaround buffer issue")
			}
			caster.Publish(append(nalDelimiter, s.Bytes()...))
			//log.Println("NAL packet bytes: " + strconv.Itoa(len(s.Bytes())))
		}
	}
}
