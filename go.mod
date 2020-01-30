module simple-webcam

go 1.13

require (
	github.com/gorilla/websocket v1.4.1
	simple-webcam/broker v0.0.0-00010101000000-000000000000
	simple-webcam/helper v0.0.0-00010101000000-000000000000
	simple-webcam/raspivid v0.0.0-00010101000000-000000000000
)

replace simple-webcam/broker => ./pkg/broker

replace simple-webcam/helper => ./pkg/helper

replace simple-webcam/raspivid => ./pkg/raspivid
