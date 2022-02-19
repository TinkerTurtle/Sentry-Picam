package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"time"

	//_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"

	"sentry-picam/broker"
	h "sentry-picam/helper"
	"sentry-picam/raspivid"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// ProductName string
const ProductName = "sentry-picam"

// ProductVersion #
const ProductVersion = "0.7.4"

var clients = make(map[*websocket.Conn]bool)
var clientsMotion = make(map[*websocket.Conn]bool)
var camera raspivid.Camera
var motion raspivid.Motion

var recorder raspivid.Recorder

//go:embed www
var staticAssets embed.FS

func streamVideoToWS(ws *websocket.Conn, caster *broker.Broker, quit chan bool) {
	stream := caster.Subscribe()
	var x interface{}
	//f, _ := os.Create("temp.h264")
	for {
		select {
		case <-quit:
			log.Println("Ending a WS video stream")
			caster.Unsubscribe(stream)
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

func streamMotionToWS(ws *websocket.Conn, caster *broker.Broker, quit chan bool) {
	stream := caster.Subscribe()
	var x interface{}
	//f, _ := os.Create("motion.vec")
	for {
		select {
		case <-quit:
			log.Println("Ending a WS motion stream")
			caster.Unsubscribe(stream)
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
		Action  string `json:"action"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
		MBwidth int    `json:"mbWidth"`
	}

	settings := initVideo{
		"init",
		*camera.Width,
		*camera.Height,
		motion.BlockWidth,
	}

	message, err := json.Marshal(settings)
	h.CheckError(err)
	//log.Println("Initializing client with: " + string(message))

	ws.WriteMessage(websocket.TextMessage, message)
}

func initClientMotion(ws *websocket.Conn) {
	type initMotion struct {
		Mask []int8 `json:"mask"`
	}

	out := make([]int8, len(motion.MotionMask))
	for i, v := range motion.MotionMask {
		out[i] = int8(v)
	}

	settings := initMotion{
		out,
	}

	message, err := json.Marshal(settings)
	h.CheckError(err)
	//log.Println("Initializing client motion with: " + string(message))

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
				//log.Println(err)
				quit <- true
				break
			}

			if messageType == websocket.TextMessage {
				log.Println("Message Received: " + string(p))
				switch string(p) {
				case "start":
					if !requestStreamStatus {
						requestStreamStatus = true
						go streamVideoToWS(ws, caster, quit)
					} else {
						log.Println("Already requested stream")
					}
				case "stop":
					quit <- true
					requestStreamStatus = false
				case "mode:night":
					camera.CameraNightMode <- true
				case "mode:day":
					camera.CameraNightMode <- false
				case "startrecord":
					recorder.RequestedRecord = true
				case "stoprecord":
					recorder.RequestedRecord = false
				}
			}
		}
	})
}

func wsHandlerMotion(caster *broker.Broker) http.Handler {
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

		clientsMotion[ws] = true
		log.Println("Client Connected for motion")

		initClientMotion(ws)

		quit := make(chan bool)
		requestStreamStatus := false
		for {
			// read in a message
			messageType, p, err := ws.ReadMessage()
			//		log.Println("Message Type: " + strconv.Itoa(messageType))
			if err != nil {
				delete(clientsMotion, ws)
				//log.Println(err)
				quit <- true
				break
			}

			if messageType == websocket.TextMessage {
				//log.Println("Motion Message Received: " + string(p))
				switch string(p) {
				case "start":
					if !requestStreamStatus {
						requestStreamStatus = true
						go streamMotionToWS(ws, caster, quit)
					} else {
						log.Println("Already requested motion stream")
					}
				case "stop":
					quit <- true
					requestStreamStatus = false
				}
			} else {
				//log.Println("Applying motion detection mask")
				motion.ApplyMask(p)
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

		quit := r.Context().Done()

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
				if !seenHeader && x.([]byte)[4] == 39 { // SPS header
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
	version := flag.Bool("version", false, "Show version")
	port := flag.Int("port", 8080, "Port to listen on.\nX+1 and X+2 ports are also used with raspivid")
	camera.Width = flag.Int("width", 1280, "Video width")
	camera.Height = flag.Int("height", 960, "Video height. 1080 needs to be 1088 for motion detection.")
	camera.Fps = flag.Int("fps", 12, "Video framerate. Minimum 1 fps")
	camera.SensorMode = flag.Int("sensor", 0, "Sensor mode")
	camera.Bitrate = flag.Int("bitrate", 2000000, "Video bitrate")
	minFreeSpace := flag.Uint64("minFreeSpace", 1073741824, "Keep at least minFreeSpace available by deleting old recordings")

	camera.ExposureValue = flag.Int("ev", 3, "(raspivid) Exposure Value")
	camera.MeteringMode = flag.String("mm", "backlit", "(raspivid) Metering Mode")
	camera.DynamicRangeCompression = flag.String("drc", "high", "(raspivid) Dynamic Range Compression")
	camera.ImageEffect = flag.String("ifx", "denoise", "(raspivid) Image Effect")
	camera.ExposureMode = flag.String("ex", "backlight", "(raspivid) Exposure Mode")

	camera.Rotation = flag.Int("rot", 0, "Rotate 0, 90, 180, or 270 degrees")
	camera.DisableMotion = flag.Bool("disablemotion", false, "Disable motion detection. Lowers CPU usage.")
	record := flag.Bool("record", false, "Record detected motion events.")
	camera.Protocol = "tcp"
	camera.ListenPort = ":" + strconv.Itoa(*port+1)
	camera.ListenPortMotion = ":" + strconv.Itoa(*port+2)

	//mNumInspectFrames := flag.Int("mframes", 3, "Number of motion frames to examine. Minimum 2.\nLower # increases sensitivity.")
	mThreshold := flag.Int("mthreshold", 9, "Motion sensitivity.\nLower # increases sensitivity.")
	mBlockWidth := flag.Int("mblockwidth", 0, "Width of motion detection block.\nVideo width and height be divisible by mblockwidth * 16\nLower # increases detection resolution")
	usePrevMotionMask := flag.Bool("upmm", false, "Use previous motion mask")
	triggerScript := flag.String("run", "", "Run script when motion is detected")
	flag.Parse()

	if *version {
		fmt.Println(ProductName + " version " + ProductVersion)
		return
	}

	log.Println(ProductName + " version " + ProductVersion)
	//motion.NumInspectFrames = *mNumInspectFrames
	motion.SenseThreshold = int8(*mThreshold)
	motion.BlockWidth = *mBlockWidth

	listenPort := ":" + strconv.Itoa(*port)
	if *camera.Bitrate < 1 || *camera.Fps < 1 {
		log.Fatal("FPS and bitrate must be greater than 1")
	}

	exDir, _ := os.Executable()
	exDir = filepath.Dir(exDir)

	recordingFolder := exDir + "/www/recordings/"

	// setup motion detector
	motion.Protocol = "tcp"
	motion.ListenPort = camera.ListenPortMotion
	motion.Width = *camera.Width
	motion.Height = *camera.Height
	motion.RecordingFolder = recordingFolder
	motion.Init(*usePrevMotionMask)

	// start broadcaster and camera
	castVideo := broker.New()
	castMotion := broker.New()
	go castVideo.Start()
	go castMotion.Start()

	go motion.Start(castMotion, &recorder)
	go camera.Start(castVideo)
	recorder.MinFreeSpace = *minFreeSpace
	go recorder.Init(castVideo, recordingFolder, *camera.Fps, *triggerScript)
	go update(recordingFolder)

	if *record {
		time.AfterFunc(2*time.Second, func() { // let raspivid settle in
			log.Println("Recording enabled from console")
			recorder.RequestedRecord = true
		})
	}

	// setup web services
	r := mux.NewRouter()
	//fs := http.FileServer(http.Dir(exDir + "/www"))
	//r.Handle("/", fs)
	r.Handle("/ws/video", wsHandler(castVideo))
	r.Handle("/ws/motion", wsHandlerMotion(castMotion))
	r.Handle("/video.h264", httpStreamHandler(castVideo))

	recordingList := RecordingList{}
	recordingList.Folder = recordingFolder
	status := Status{}
	status.Recorder = &recorder
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/videos", recordingList.handleRecordingList).Methods("GET")
	api.HandleFunc("/videos/cleanup", recordingList.handleDestroyRecording).Methods("DELETE")
	api.HandleFunc("/videos/{videoID}", recordingList.handleDeleteRecording).Methods("DELETE")
	//api.HandleFunc("/videos/{videoID}/thumbnail", recordingList.handleThumbnailUpdate).Methods("POST")
	api.HandleFunc("/status", status.handleStatus).Methods("GET")

	// static files
	//r.PathPrefix("/js/").Handler(http.StripPrefix("/js/", http.FileServer(http.Dir(exDir+"/www/js"))))
	os.MkdirAll(recordingFolder, 0700)
	r.PathPrefix("/recordings/").Handler(http.StripPrefix("/recordings/", http.FileServer(http.Dir(exDir+"/www/recordings"))))
	webRoot, _ := fs.Sub(staticAssets, "www")
	r.PathPrefix("/").Handler(http.FileServer(http.FS(webRoot)))

	log.Println("HTTP Listening on " + listenPort)
	http.ListenAndServe(listenPort, r)
}
