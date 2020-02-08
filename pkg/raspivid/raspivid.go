package raspivid

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"os/exec"
	"strconv"
	"time"

	"simple-webcam/broker"
	h "simple-webcam/helper"
)

// Camera is a wrapper for raspivid
type Camera struct {
	Width, Height, Fps, Bitrate, SensorMode, Rotation *int
	CameraNightMode                                   chan bool
	Protocol                                          string
	ListenPort                                        string
	ListenPortMotion                                  string
}

func (c *Camera) getRaspividArgs() []string {
	return []string{
		"-t", "0",
		"-o", c.Protocol + "://127.0.0.1" + c.ListenPort,
		"-x", c.Protocol + "://127.0.0.1" + c.ListenPortMotion,
		"-w", strconv.Itoa(*c.Width),
		"-h", strconv.Itoa(*c.Height),
		"-rot", strconv.Itoa(*c.Rotation),
		"-ev", "3",
		"-mm", "backlit",
		"-drc", "high",
		"-ifx", "denoise",
		"-b", strconv.Itoa(*c.Bitrate),
		"-md", strconv.Itoa(*c.SensorMode),
		"-pf", "baseline",
		"-g", strconv.Itoa(*c.Fps * 2),
		"-ih", //"-stm",
		"-a", "1028",
		"-a", "%Y-%m-%d %l:%M:%S %P",
	}
}

func (c *Camera) startDayCamera() (io.ReadCloser, *exec.Cmd) {
	args := c.getRaspividArgs()
	args = append(args,
		"-fps", strconv.Itoa(*c.Fps),
		"-ex", "backlight",
	)
	cmd := exec.Command("raspivid", args...)
	stdOut, err := cmd.StdoutPipe()
	h.CheckError(err)

	return stdOut, cmd
}

func (c *Camera) startNightCamera() (io.ReadCloser, *exec.Cmd) {
	args := c.getRaspividArgs()
	args = append(args,
		"-fps", "0",
		"-ex", "nightpreview",
	)
	cmd := exec.Command("raspivid", args...)
	stdOut, err := cmd.StdoutPipe()
	h.CheckError(err)

	return stdOut, cmd
}

func (c *Camera) receiveStream(reader chan net.Conn) {
	reader <- listen(c.Protocol, c.ListenPort)
}

func stopRaspivid(cmd *exec.Cmd, conn net.Conn) {
	conn.Close()
	err := cmd.Process.Kill()
	h.CheckError(err)
	cmd.Wait()

	time.Sleep(time.Second) // hacky way to give raspivid time to shut down. maybe i'm not sending the right signal?
}

func (c *Camera) startStream(caster *broker.Broker) {
	c.CameraNightMode = make(chan bool)
	stream := make(chan net.Conn)
	go c.receiveStream(stream)

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

	_, cmd := c.startDayCamera()
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	log.Println("Camera Online")

	buffer := make([]byte, *c.Bitrate/5)

	conn := <-stream
	s := bufio.NewScanner(conn)
	s.Buffer(buffer, len(buffer))
	s.Split(splitFunc)

	for {
		select {
		case nightMode := <-c.CameraNightMode:
			if nightMode {
				log.Println("Switching to night mode")
				stopRaspivid(cmd, conn)

				_, cmd = c.startNightCamera()
				if err := cmd.Start(); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Println("Switching to day mode")
				stopRaspivid(cmd, conn)

				_, cmd = c.startDayCamera()
				if err := cmd.Start(); err != nil {
					log.Fatal(err)
				}
			}
		default:
			if s.Scan() == false {
				log.Println("Stream interrupted")
				return
			}
			if len(s.Bytes()) > 0 {
				caster.Publish(append(nalDelimiter, s.Bytes()...))
				//log.Println("NAL packet bytes: " + strconv.Itoa(len(s.Bytes())))
			}
		}
	}
}

// Start initializes the broadcast channel and starts raspivid
func (c *Camera) Start(caster *broker.Broker, recordButton *Recorder) {
	for {
		c.startStream(caster)
	}
}
