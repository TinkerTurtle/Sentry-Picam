package helper

import "log"

// CheckError prints errors to the standard log
func CheckError(err error) {
	if err != nil {
		log.Println(err)
	}
}
