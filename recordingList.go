package main

import (
	"encoding/json"
	"fmt"
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

func getFiles(folder string) []string {
	files, err := os.ReadDir(folder)
	if err != nil {
		log.Fatal(err)
	}

	var recordings []string
	for _, f := range files {
		extension := filepath.Ext(strings.ToLower(f.Name()))
		name := strings.TrimSuffix(filepath.Base(f.Name()), filepath.Ext(f.Name()))

		if extension == ".jpg" {
			s := strings.Split(f.Name(), "-")
			newFolder := fmt.Sprintf("%s-%s/", s[0], s[1])
			recordings = append(recordings, newFolder+name)
		}
	}

	return recordings
}

func (rec *RecordingList) handleRecordingList(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(rec.Folder)
	if err != nil {
		log.Fatal(err)
	}

	var recordings []string
	for _, f := range files {
		if f.IsDir() && f.Name() != "deleteme" && f.Name() != "raw" {
			recordings = append(recordings, getFiles(rec.Folder+f.Name())...)
		}
	}

	out, _ := json.Marshal(recordings)
	w.Header().Set("Content-Type", "application/json")
	w.Write(out)
}

func (rec *RecordingList) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID := vars["videoID"]

	os.MkdirAll(rec.Folder+"deleteme/", 0777)

	s := strings.Split(videoID, "-")
	newFolder := fmt.Sprintf("%s-%s/", s[0], s[1])
	os.Rename(rec.Folder+newFolder+videoID+".mp4", rec.Folder+"deleteme/"+videoID+".mp4")
	os.Rename(rec.Folder+newFolder+videoID+".jpg", rec.Folder+"deleteme/"+videoID+".jpg")
}

func (rec *RecordingList) handleDestroyRecording(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(rec.Folder + "deleteme/")
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
