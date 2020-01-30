package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"simple-webcam/broker"
	h "simple-webcam/helper"
	"simple-webcam/raspivid"

	"github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)
var camera raspivid.Camera

func streamVideoToWS(ws *websocket.Conn, caster *broker.Broker, quit chan bool) {
	stream := caster.Subscribe()
	var x interface{}
	//f, _ := os.Create("temp.h264")
	for {
		select {
		case <-quit:
			log.Println("Ending a WS video stream")
			return
		default:
			x = <-stream
			ws.WriteMessage(websocket.BinaryMessage, x.([]byte))
			//f.Write(x.([]byte))
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

	settings := initVideo{
		"init",
		*camera.Width,
		*camera.Height,
	}

	message, err := json.Marshal(settings)

	log.Println("Initializing client with: " + string(message))
	h.CheckError(err)

	ws.WriteMessage(websocket.TextMessage, message)
}

func wsHandler(caster *broker.Broker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		upgrader.CheckOrigin = func(r *http.Request) bool { return true }

		// upgrade this connection to a WebSocket connection
		ws, err := upgrader.Upgrade(w, r, nil)
		h.CheckError(err)
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
						go streamVideoToWS(ws, caster, quit)
					} else {
						log.Println("Already requested stream")
					}
				case "STOPSTREAM":
					quit <- true
					requestStreamStatus = false
				case "NIGHTMODE":
					camera.CameraNightMode <- true
				case "DAYMODE":
					camera.CameraNightMode <- false
				}
			}
		}
	})
}

func httpStreamHandler(caster *broker.Broker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Starting HTTP stream")
		w.Header().Add("Content-Type", "video/H264")
		w.Header().Add("Transfer-Encoding", "chunked")
		w.WriteHeader(200)

		quit := w.(http.CloseNotifier).CloseNotify()

		seenHeader := false
		stream := caster.Subscribe()
		var x interface{}
	loop:
		for {
			select {
			case <-quit:
				break loop
			default:
				x = <-stream
				if seenHeader == false && x.([]byte)[4] == 39 { // SPS header
					seenHeader = true
				}

				if seenHeader {
					w.Write(x.([]byte))
				}
			}
		}

		log.Println("Ending HTTP stream")
	})
}

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	camera.Width = flag.Int("width", 1280, "Video width")
	camera.Height = flag.Int("height", 960, "Video height")
	camera.Fps = flag.Int("fps", 12, "Video framerate. Minimum 1 fps")
	camera.SensorMode = flag.Int("sensor", 0, "Sensor mode")
	camera.Bitrate = flag.Int("bitrate", 1500000, "Video bitrate")
	camera.Rotation = flag.Int("rot", 0, "Rotate 0, 90, 180, or 270 degrees")
	flag.Parse()

	listenPort := ":" + strconv.Itoa(*port)
	if *camera.Bitrate < 1 || *camera.Fps < 1 {
		log.Fatal("FPS and bitrate must be greater than 1")
	}

	// start broadcaster and camera
	caster := broker.New()
	go camera.Start(caster)
	go caster.Start()

	// setup web services
	exDir, _ := os.Executable()
	exDir = filepath.Dir(exDir)
	fs := http.FileServer(http.Dir(exDir + "/www"))
	http.Handle("/", fs)
	http.Handle("/ws", wsHandler(caster))
	http.Handle("/video.h264", httpStreamHandler(caster))

	log.Println("Listening on " + listenPort)
	http.ListenAndServe(listenPort, nil)
}
