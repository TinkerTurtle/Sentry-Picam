module simple-webcam/raspivid

go 1.13

require (
	simple-webcam/broker v0.0.0-00010101000000-000000000000
	simple-webcam/helper v0.0.0-00010101000000-000000000000
)

replace simple-webcam/helper => ../helper

replace simple-webcam/broker => ../broker
