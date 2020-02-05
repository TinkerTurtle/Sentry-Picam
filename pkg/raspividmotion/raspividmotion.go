package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"

	"simple-webcam/broker"
)

const ignoreFirstFrames = 10 // give camera's autoexposure some time to settle
// Motion stores configuration parameters and forms the basis for Detect
type Motion struct {
	Width          int
	Height         int
	NumAvgFrames   int
	SenseThreshold int8
	BlockWidth     int
	Protocol       string
	ListenPort     string
}

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

func reportFrameAvgDiff(buf *[]motionVector, numBlocks int) {
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

// buildAvgBlock takes a blockWidth * blockWidth average of macroblocks from buf and stores the
// condensed result into frame
func (c *Motion) buildAvgBlocks(frame *[]motionVector, buf *[]motionVector) {
	rowCount := c.Height / 16
	colCount := (c.Width + 16) / 16
	usableCols := colCount - 1

	mV := make([]mVhelper, usableCols/c.BlockWidth)
	i := 0
	compressedIndex := 0

	for x := 0; x < rowCount; x++ {
		blk := 1
		blkIndex := 0
		for y := 0; y < colCount; y++ {
			if y < usableCols {
				mV[blkIndex].add((*buf)[i])
			}
			if blk == c.BlockWidth {
				blk = 0
				blkIndex++
			}
			blk++
			i++
		}
		if x%c.BlockWidth == 0 {
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

// findTemporalAverage examines the past numFrames and stores the average to frameAvg
func (c *Motion) findTemporalAverage(frameAvg *[]motionVector, frameHistory *[][]motionVector, currFrame *[]motionVector) {
	var mV mVhelper

	if len(*frameHistory) >= c.NumAvgFrames {
		for i := 0; i < len(*currFrame); i++ {
			for j := 0; j < c.NumAvgFrames; j++ {
				mV.add((*frameHistory)[j][i])
			}
			(*frameAvg)[i] = mV.getAvg()
			mV.reset()
		}
		*frameHistory = (*frameHistory)[1:] // slice off oldest frame
	}
	var f2 = make([]motionVector, len(*currFrame))
	copy(f2, *currFrame)
	*frameHistory = append(*frameHistory, f2)
}

// listen starts listening for raspivid's output of motion vectors
func listen(network string, sockAddr string) net.Conn {
	l, err := net.Listen(network, sockAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Wait for a connection.
	conn, err := l.Accept()
	if err != nil {
		log.Fatal(err)
	}

	return conn
}

func (c *Motion) getMaxBlockWidth() {
	sizeMacroX := c.Width / 16
	sizeMacroY := c.Height / 16

	// split frame into larger zones, find largest factor
	simpFactor := 0
	for ((sizeMacroX/(1<<simpFactor))%2 == 0) && ((sizeMacroY/(1<<simpFactor))%2 == 0) {
		simpFactor++
	}
	blockWidth := 1 << simpFactor // largest block width

	if c.BlockWidth != 0 {
		blockWidth = c.BlockWidth
	}
	c.BlockWidth = blockWidth
}

func (c *Motion) reportChanges(frameAvg *[]motionVector, currFrame *[]motionVector) {
	pRowIdx := 0
	for i, v := range *frameAvg {
		pRowIdx++
		if abs(v.X-(*currFrame)[i].X) > c.SenseThreshold || abs(v.Y-(*currFrame)[i].Y) > c.SenseThreshold {
			fmt.Print("X")
		} else {
			fmt.Print(".")
		}
		if pRowIdx > 19 {
			fmt.Println()
			pRowIdx = 0
		}
	}
	reportFrameAvgDiff(frameAvg, len(*frameAvg))
	reportFrameAvgDiff(currFrame, len(*currFrame))
}

func (c *Motion) init() {
	if c.Width == 0 || c.Height == 0 {
		c.Width = 1280
		c.Height = 960
	}

	if c.NumAvgFrames == 0 {
		c.NumAvgFrames = 5
	}

	if c.SenseThreshold == 0 {
		c.SenseThreshold = 6
	}

	if c.Protocol == "" || c.ListenPort == "" {
		c.Protocol = "tcp"
		c.ListenPort = ":9000"
	}
}

func (c *Motion) publish(caster *broker.Broker, frameAvg *[]motionVector, currFrame *[]motionVector) {
	out := ""
	pRowIdx := 0
	for i, v := range *frameAvg {
		pRowIdx++
		if abs(v.X-(*currFrame)[i].X) > c.SenseThreshold || abs(v.Y-(*currFrame)[i].Y) > c.SenseThreshold {
			out += "X"
		} else {
			out += "."
		}
		if pRowIdx > 19 {
			out += "\n"
			pRowIdx = 0
		}
	}
	reportFrameAvgDiff(frameAvg, len(*frameAvg))
	reportFrameAvgDiff(currFrame, len(*currFrame))
	caster.Publish(out)
}

// Detect motion by comparing the most recent frame with an average of the past numFrames.
// Lower senseThreshold value increases the sensitivity to motion.
func (c *Motion) Detect(caster *broker.Broker) {
	c.init()
	conn := listen(c.Protocol, c.ListenPort)

	//f, _ := os.Create("motion.vec")
	numMacroblocks := ((c.Width + 16) / 16) * (c.Height / 16) // the right-most column is padding?
	numUsableMacroblocks := (c.Width / 16) * (c.Height / 16)

	c.getMaxBlockWidth()

	buffer := make([]motionVector, numMacroblocks)
	currVectorBlocks := make([]motionVector, numUsableMacroblocks/(c.BlockWidth*c.BlockWidth))
	vectorAvgBlocks := make([]motionVector, numUsableMacroblocks/(c.BlockWidth*c.BlockWidth))
	vectorHistory := make([][]motionVector, 0, c.NumAvgFrames)
	ignoredFrames := 0
	for {
		err := binary.Read(conn, binary.LittleEndian, &buffer)
		if err != nil {
			log.Println("Motion detection stopped: " + err.Error())
			return
		}

		if ignoredFrames < ignoreFirstFrames {
			ignoredFrames++
			continue
		}
		c.buildAvgBlocks(&currVectorBlocks, &buffer)
		c.findTemporalAverage(&vectorAvgBlocks, &vectorHistory, &currVectorBlocks)
		//c.reportChanges(&vectorAvgBlocks, &currVectorBlocks)
		c.publish(caster, &vectorAvgBlocks, &currVectorBlocks)

		//binary.Write(f, binary.LittleEndian, &buffer) // write to file
	}
}

// Start motion detection and continues listening after interruptions to the data stream
func (c *Motion) Start(caster *broker.Broker) {
	for {
		c.Detect(caster)
	}
}

func main() {
	cam := Motion{}

	castMotion := broker.New()
	go castMotion.Start()
	go cam.Start(castMotion)

	reader := castMotion.Subscribe()
	for {
		fmt.Println(<-reader)
	}
}
