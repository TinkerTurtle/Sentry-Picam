package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/gorilla/websocket"
)

var width, height, fps, bitrate, sensorMode *int
var bitrate2 *int

var clients = make(map[*websocket.Conn]bool)
var broker = NewBroker()
var cameraNightMode = make(chan bool)

// Broker implementation from https://stackoverflow.com/a/49877632
type Broker struct {
	stopCh    chan struct{}
	publishCh chan interface{}
	subCh     chan chan interface{}
	unsubCh   chan chan interface{}
}

func NewBroker() *Broker {
	return &Broker{
		stopCh:    make(chan struct{}),
		publishCh: make(chan interface{}, 1),
		subCh:     make(chan chan interface{}, 1),
		unsubCh:   make(chan chan interface{}, 1),
	}
}

func (b *Broker) Start() {
	subs := map[chan interface{}]struct{}{}
	for {
		select {
		case <-b.stopCh:
			return
		case msgCh := <-b.subCh:
			subs[msgCh] = struct{}{}
		case msgCh := <-b.unsubCh:
			delete(subs, msgCh)
		case msg := <-b.publishCh:
			for msgCh := range subs {
				// msgCh is buffered, use non-blocking send to protect the broker:
				select {
				case msgCh <- msg:
				default:
				}
			}
		}
	}
}

func (b *Broker) Stop() {
	close(b.stopCh)
}

func (b *Broker) Subscribe() chan interface{} {
	msgCh := make(chan interface{}, 5)
	b.subCh <- msgCh
	return msgCh
}

func (b *Broker) Unsubscribe(msgCh chan interface{}) {
	b.unsubCh <- msgCh
}

func (b *Broker) Publish(msg interface{}) {
	b.publishCh <- msg
}

func checkError(err error) {
	if err != nil {
		log.Println(err)
	}
}

func streamVideo(ws *websocket.Conn, quit chan bool) {
	stream := broker.Subscribe()
	var x interface{}
	//	f, _ := os.Create("temp.h264")
	for {
		select {
		case <-quit:
			return
		default:
			x = <-stream
			ws.WriteMessage(websocket.BinaryMessage, x.([]byte))
			//		f.Write(x.([]byte))
			//		log.Println("sending--------------\n" + hex.Dump(x.([]byte)))
			//		log.Println("sent-----------------")
			//log.Println("sent bytes: " + strconv.Itoa(len(x.([]byte))))
		}
	}
}

func initClientVideo(ws *websocket.Conn) {
	type initVideo struct {
		Action string `json:"action"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	message, err := json.Marshal(initVideo{
		Action: "init",
		Height: *height,
		Width:  *width,
	})
	log.Println("Initializing client with: " + string(message))
	checkError(err)

	ws.WriteMessage(1, message)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// upgrade this connection to a WebSocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	checkError(err)
	defer ws.Close()

	clients[ws] = true
	log.Println("Client Connected")

	initClientVideo(ws)

	quit := make(chan bool)
	requestStreamStatus := false
	for {
		// read in a message
		messageType, p, err := ws.ReadMessage()
		//		log.Println("Message Type: " + strconv.Itoa(messageType))
		if err != nil {
			delete(clients, ws)
			log.Println(err)
			quit <- true
			break
		}

		if messageType == websocket.TextMessage {
			log.Println("Message Received: " + string(p))
			switch string(p) {
			case "REQUESTSTREAM":
				if !requestStreamStatus {
					requestStreamStatus = true
					go streamVideo(ws, quit)
				} else {
					log.Println("Already requested stream")
				}
			case "STOPSTREAM":
				quit <- true
				requestStreamStatus = false
			case "NIGHTMODE":
				cameraNightMode <- true
			case "DAYMODE":
				cameraNightMode <- false
			}
		}
	}
}

func startDayCamera() (io.ReadCloser, *exec.Cmd) {
	cmd := exec.Command("raspivid",
		"-t", "0",
		"-o", "-",
		"-w", strconv.Itoa(*width),
		"-h", strconv.Itoa(*height),
		"-fps", strconv.Itoa(*fps),
		"-drc", "high",
		"-b", strconv.Itoa(*bitrate),
		"-md", strconv.Itoa(*sensorMode),
		"-pf", "baseline",
		"-g", strconv.Itoa(*fps*2),
		"-ih",
		"-a", "1028",
		"-a", " %Y-%m-%d %l:%M:%S %P",
	)
	stdOut, err := cmd.StdoutPipe()
	checkError(err)

	return stdOut, cmd
}

func startNightCamera() (io.ReadCloser, *exec.Cmd) {
	cmd := exec.Command("raspivid",
		"-t", "0",
		"-o", "-",
		"-w", strconv.Itoa(*width),
		"-h", strconv.Itoa(*height),
		"-fps", "0",
		"-ex", "nightpreview",
		"-drc", "high",
		"-b", strconv.Itoa(*bitrate),
		"-md", strconv.Itoa(*sensorMode),
		"-pf", "baseline",
		"-g", strconv.Itoa(*fps*2),
		"-ih",
		"-a", "1028",
		"-a", " %Y-%m-%d %l:%M:%S %P",
	)
	stdOut, err := cmd.StdoutPipe()
	checkError(err)

	return stdOut, cmd
}

func cameraSupervisor() {
	searchBytes := []byte{0, 0, 0, 1}
	searchLen := len(searchBytes)
	splitFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Return nothing if at end of file and no data passed
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		// Find the index of the NAL header
		if i := bytes.Index(data, searchBytes); i >= 0 {
			return i + searchLen, data[0:i], nil
		}

		// If at end of file with data return the data
		if atEOF {
			return len(data), data, nil
		}

		return
	}

	stdOut, cmd := startDayCamera()
	log.Println("Camera Online")

	buffer := make([]byte, *bitrate/5)
	s := bufio.NewScanner(io.Reader(stdOut))
	s.Buffer(buffer, len(buffer))
	s.Split(splitFunc)
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case nightMode := <-cameraNightMode:
			if nightMode {
				log.Println("Switching to night mode")
				err := cmd.Process.Kill()
				checkError(err)

				stdOut, cmd = startNightCamera()
				s = bufio.NewScanner(io.Reader(stdOut))
				s.Buffer(buffer, len(buffer))
				s.Split(splitFunc)
				if err := cmd.Start(); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Println("Switching to day mode")
				err := cmd.Process.Kill()
				checkError(err)

				stdOut, cmd = startDayCamera()
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
			broker.Publish(append(searchBytes, s.Bytes()...))
			//log.Println("NAL packet bytes: " + strconv.Itoa(len(s.Bytes())))
		}
	}
}

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	width = flag.Int("width", 1280, "Video width")
	height = flag.Int("height", 960, "Video height")
	fps = flag.Int("fps", 12, "Video framerate. Minimum 1 fps")
	sensorMode = flag.Int("sensor", 0, "Sensor mode")
	bitrate = flag.Int("bitrate", 1500000, "Video bitrate")
	flag.Parse()

	listenPort := ":" + strconv.Itoa(*port)
	if *bitrate < 1 || *fps < 1 {
		log.Fatal("FPS and bitrate must be greater than 1")
	}

	go cameraSupervisor()
	go broker.Start()

	// start services
	exDir, _ := os.Executable()
	exDir = filepath.Dir(exDir)
	fs := http.FileServer(http.Dir(exDir + "/www"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", wsHandler)

	log.Println("Listening on " + listenPort)
	http.ListenAndServe(listenPort, nil)
}
