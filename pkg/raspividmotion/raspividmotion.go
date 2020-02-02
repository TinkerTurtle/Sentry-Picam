package main

import (
	"encoding/binary"
	"log"
	"net"
)

type motionVector struct {
	Xvector int8
	Yvector int8
	SAD     int16 // Sum of Absolute Difference
}

func getFrameAvgDiff(buf *[]motionVector, numBlocks int) {
	var totalSAD, totalX, totalY int32
	var maxSAD int16
	var maxX, maxY int8
	for _, v := range *buf {
		totalSAD += int32(v.SAD)
		totalX += int32(v.Yvector)
		totalY += int32(v.Xvector)
		if v.SAD > maxSAD {
			maxSAD = v.SAD
		}
		if v.Yvector > maxY {
			maxY = v.Yvector
		}
		if v.Xvector > maxX {
			maxX = v.Xvector
		}
	}

	log.Printf("Blocks: %5d SAD: %7d %7d X: %5d %5d Y: %5d %5d\n",
		numBlocks,
		totalSAD/int32(numBlocks), maxSAD,
		totalX/int32(numBlocks), maxX,
		totalY/int32(numBlocks), maxY)
}

func main() {
	width := 1280
	height := 960
	//sockAddr := "/dev/shm/simple-webcam.sock"
	//l, err := net.Listen("unix", sockAddr)
	sockAddr := ":9000"
	l, err := net.Listen("tcp", sockAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Wait for a connection.
	log.Println("Waiting...")
	conn, err := l.Accept()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected.")

	//f, _ := os.Create("motion.vec")
	//frameSize := ((width + 16) / 16) * (height / 16)
	frameSize := (width / 16) * (height / 16)

	buffer := make([]motionVector, frameSize)
	for {
		err := binary.Read(conn, binary.LittleEndian, &buffer)
		if err != nil {
			log.Println(err)
			return
		}
		//		log.Println(buffer)
		getFrameAvgDiff(&buffer, len(buffer))

		//binary.Write(f, binary.LittleEndian, &buffer) // write to file
	}
}
