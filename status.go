package main

import (
	"encoding/json"
	"net/http"
	"sentry-picam/raspivid"
)

type Status struct {
	Recorder *raspivid.Recorder
}

func (rec *Status) handleStatus(w http.ResponseWriter, r *http.Request) {
	var settings struct {
		RecordingStatus int `json:"isRecording"`
	}
	if rec.Recorder.RequestedRecord {
		settings.RecordingStatus = 1
	} else {
		settings.RecordingStatus = 0
	}

	out, _ := json.Marshal(settings)
	w.Write(out)
}
