module sentry-picam

go 1.13

require (
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/ricochet2200/go-disk-usage/du v0.0.0-20210707232629-ac9918953285 // indirect
	sentry-picam/broker v0.0.0-00010101000000-000000000000
	sentry-picam/helper v0.0.0-00010101000000-000000000000
	sentry-picam/raspivid v0.0.0-00010101000000-000000000000
)

replace sentry-picam/broker => ./pkg/broker

replace sentry-picam/helper => ./pkg/helper

replace sentry-picam/raspivid => ./pkg/raspivid
