package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type RecordingList struct {
	Folder string
}

func (rec *RecordingList) handleRecordingList(w http.ResponseWriter, r *http.Request) {
	files, err := ioutil.ReadDir(rec.Folder)
	if err != nil {
		log.Fatal(err)
	}

	var recordings []string
	for _, f := range files {
		extension := filepath.Ext(strings.ToLower(f.Name()))
		name := strings.TrimSuffix(filepath.Base(f.Name()), filepath.Ext(f.Name()))

		if extension == ".jpg" {
			recordings = append(recordings, name)
		}
	}

	out, _ := json.Marshal(recordings)
	w.Write(out)
}

func (rec *RecordingList) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID := vars["videoID"]

	os.MkdirAll(rec.Folder+"deleteme/", 0777)

	os.Rename(rec.Folder+videoID+".mp4", rec.Folder+"deleteme/"+videoID+".mp4")
	os.Rename(rec.Folder+videoID+".jpg", rec.Folder+"deleteme/"+videoID+".jpg")
}

func (rec *RecordingList) handleDestroyRecording(w http.ResponseWriter, r *http.Request) {
	files, err := ioutil.ReadDir(rec.Folder + "deleteme/")
	if err != nil {
		return
	}

	for _, f := range files {
		s := strings.Split(f.Name(), "_")

		videoTime, _ := time.Parse("2006-01-02-1504", s[0])
		if time.Now().Add(-time.Hour * 24 * 7).After(videoTime) {
			os.Remove(rec.Folder + "deleteme/" + f.Name())
		}
	}
}
