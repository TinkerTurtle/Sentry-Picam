package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

type motionVector struct {
	X   int8
	Y   int8
	SAD int16 // Sum of Absolute Difference
}

type mVhelper struct {
	X     int
	Y     int
	SAD   int
	count int
}

func (mV *mVhelper) add(v motionVector) {
	mV.count++
	mV.X += int(v.X)
	mV.Y += int(v.Y)
	mV.SAD += int(v.SAD)
}

func (mV *mVhelper) getAvg() motionVector {
	return motionVector{
		int8(mV.X / mV.count),
		int8(mV.Y / mV.count),
		int16(mV.SAD / mV.count),
	}
}

func (mV *mVhelper) reset() {
	mV.count = 0
	mV.X = 0
	mV.Y = 0
	mV.SAD = 0
}

func buildFrameAvgDiff(buf *[]motionVector, numBlocks int) {
	var totalSAD, totalX, totalY int32
	var maxSAD int16
	var maxX, maxY int8
	for _, v := range *buf {
		totalSAD += int32(v.SAD)
		totalX += int32(v.Y)
		totalY += int32(v.X)
		if v.SAD > maxSAD {
			maxSAD = v.SAD
		}
		if v.Y > maxY {
			maxY = v.Y
		}
		if v.X > maxX {
			maxX = v.X
		}
	}

	log.Printf("Blocks: %5d SAD: %7d %7d X: %5d %5d Y: %5d %5d\n",
		numBlocks,
		totalSAD/int32(numBlocks), maxSAD,
		totalX/int32(numBlocks), maxX,
		totalY/int32(numBlocks), maxY)
}

// buildAvgBlock takes a blockSize * blockSize average of macroblocks from buf and stores it into frame
func buildAvgBlocks(frame *[]motionVector, buf *[]motionVector, blockSize int) {
	rowCount := height / 16
	colCount := (width + 16) / 16
	usableCols := colCount - 1

	mV := make([]mVhelper, usableCols/blockSize)
	i := 0
	compressedIndex := 0

	for x := 0; x < rowCount; x++ {
		blk := 1
		blkIndex := 0
		for y := 0; y < colCount; y++ {
			if y < usableCols {
				mV[blkIndex].add((*buf)[i])
			}
			if blk == blockSize {
				blk = 0
				blkIndex++
			}
			blk++
			i++
		}
		if x%blockSize == 0 {
			for idx, v := range mV {
				(*frame)[compressedIndex] = v.getAvg()
				mV[idx].reset()
				compressedIndex++
			}
		}
	}
}

func abs(x int8) int8 {
	if x < 0 {
		return -x
	}
	return x
}

func findTemporalAverage(frameAvg *[]motionVector, frameHistory *[][]motionVector, frame []motionVector, numAvgFrames int, sensitivity int8) {
	var mV mVhelper

	if len(*frameHistory) >= numAvgFrames {
		for i := 0; i < len(frame); i++ {
			for j := 0; j < numAvgFrames; j++ {
				mV.add((*frameHistory)[j][i])
			}
			(*frameAvg)[i] = mV.getAvg()
			mV.reset()
		}
		*frameHistory = (*frameHistory)[1:] // slice off oldest frame

		c := 0
		for i, v := range *frameAvg {
			c++
			if abs(v.X-frame[i].X) > sensitivity {
				fmt.Print("X")
			} else {
				fmt.Print(".")
			}
			if c > 19 {
				fmt.Println()
				c = 0
			}
		}
		buildFrameAvgDiff(frameAvg, len(*frameAvg))
		buildFrameAvgDiff(&frame, len(frame))
	}
	var f2 = make([]motionVector, len(frame))
	copy(f2, frame)
	*frameHistory = append(*frameHistory, f2)
}

var width, height int

func main() {
	width = 1280
	height = 960
	numAvgFrames := 5            // use number of frames to find average
	sensitivity := int8(2)       // lower # is more sensitive
	const ignoreFirstFrames = 10 // give camera's autoexposure some time to settle
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
	numMacroblocks := ((width + 16) / 16) * (height / 16) // the right-most column is padding?
	numUsableMacroblocks := (width / 16) * (height / 16)
	sizeMacroX := width / 16
	sizeMacroY := height / 16
	//numMacroblocks := (width / 16) * (height / 16) // number of macroblocks in frame

	// split frame into larger zones, find largest factor
	simpFactor := 0
	for ((sizeMacroX/(1<<simpFactor))%2 == 0) && ((sizeMacroY/(1<<simpFactor))%2 == 0) {
		simpFactor++
	}
	blockSize := 1 << simpFactor // largest block width

	buffer := make([]motionVector, numMacroblocks)
	vectorBlocks := make([]motionVector, numUsableMacroblocks/(blockSize*blockSize))
	vectorAvgBlocks := make([]motionVector, numUsableMacroblocks/(blockSize*blockSize))
	vectorHistory := make([][]motionVector, 0, numAvgFrames)
	ignoredFrames := 0
	for {
		err := binary.Read(conn, binary.LittleEndian, &buffer)
		if err != nil {
			log.Println(err)
			return
		}
		//		log.Println(buffer)
		//buildFrameAvgDiff(&buffer, len(buffer))
		if ignoredFrames < ignoreFirstFrames {
			ignoredFrames++
			continue
		}
		buildAvgBlocks(&vectorBlocks, &buffer, blockSize)
		findTemporalAverage(&vectorAvgBlocks, &vectorHistory, vectorBlocks, numAvgFrames, sensitivity)

		//binary.Write(f, binary.LittleEndian, &buffer) // write to file
	}
}
