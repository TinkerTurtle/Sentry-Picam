package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// update the structure of persistent storage
func update(folder string) {
	files, err := os.ReadDir(folder)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if !f.IsDir() {
			extension := filepath.Ext(strings.ToLower(f.Name()))

			if extension == ".jpg" || extension == ".mp4" {
				s := strings.Split(f.Name(), "-")
				newFolder := fmt.Sprintf("%s/%s-%s/", folder, s[0], s[1])
				os.MkdirAll(newFolder, 0777)
				os.Rename(folder+f.Name(), newFolder+f.Name())
			}
		}
	}
}
