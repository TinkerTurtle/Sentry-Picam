module sentry-picam/raspivid

go 1.13

require (
	sentry-picam/broker v0.0.0-00010101000000-000000000000
	sentry-picam/helper v0.0.0-00010101000000-000000000000
)

replace sentry-picam/helper => ../helper

replace sentry-picam/broker => ../broker
