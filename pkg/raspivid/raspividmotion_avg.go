// +build ignore

package raspivid

/*
Usage:
	cam := Motion{}

	castMotion := broker.New()
	go castMotion.Start()
	go cam.Start(castMotion)

	reader := castMotion.Subscribe()
	for {
		fmt.Println(<-reader)
	}

*/
import (
	"bufio"
	"log"
	"time"

	"simple-webcam/broker"
)

const ignoreFirstFrames = 10 // give camera's autoexposure some time to settle
// Motion stores configuration parameters and forms the basis for Detect
type Motion struct {
	Width            int
	Height           int
	NumInspectFrames int
	SenseThreshold   int8
	BlockWidth       int
	Protocol         string
	ListenPort       string
	MotionMask       []byte
	output           []byte
	recorder         *Recorder
}

// motionVector from raspivid.
// Ignoring Y since it might be redundant
// SAD Might be unusable since it periodically spikes
const sizeofMotionVector = 4 // size of a motion vector in bytes
type motionVector struct {
	X int8
	Y int8
	//SAD int16 // Sum of Absolute Difference.
}

type mVhelper struct {
	X int
	Y int
	//SAD   int
	count int
}

func (mV *mVhelper) add(v motionVector) {
	mV.count++
	mV.X += int(v.X)
	mV.Y += int(v.Y)
	//mV.SAD += int(v.SAD)
}

func (mV *mVhelper) getAvg() motionVector {
	return motionVector{
		int8(mV.X / mV.count),
		int8(mV.Y / mV.count),
		//int16(mV.SAD / mV.count),
	}
}

func (mV *mVhelper) reset() {
	mV.count = 0
	mV.X = 0
	mV.Y = 0
	//mV.SAD = 0
}

func reportFrameAvgDiff(buf *[]motionVector, numBlocks int) {
	/*	var totalSAD, totalX, totalY int32
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
			totalY/int32(numBlocks), maxY)*/
}

// ApplyMask applies a mask to ignore specified motion blocks
func (c *Motion) ApplyMask(mask []byte) {
	c.MotionMask = mask
}

// condenseBlocksAvg takes a blockWidth * blockWidth average of macroblocks from buf and stores the
// condensed result into frame
func (c *Motion) condenseBlocksAvg(frame *[]motionVector, buf *[]motionVector) {
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

	if len(*frameHistory) >= c.NumInspectFrames {
		for i := 0; i < len(*currFrame); i++ {
			for j := 0; j < c.NumInspectFrames; j++ {
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

// logVectorHistory logs currFrame into frameHistory
func (c *Motion) logVectorHistory(frameHistory *[][]motionVector, currFrame *[]motionVector) {
	if len(*frameHistory) >= c.NumInspectFrames {
		*frameHistory = (*frameHistory)[1:] // slice off oldest frame
	}
	var f2 = make([]motionVector, len(*currFrame))
	copy(f2, *currFrame)
	*frameHistory = append(*frameHistory, f2)
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

	if c.Width%(16*c.BlockWidth) != 0 || c.Height%(16*c.BlockWidth) != 0 {
		log.Fatal("Invalid block width")
	}
}

// Init initializes configuration variables for Motion
func (c *Motion) Init() {
	if c.Width == 0 || c.Height == 0 {
		c.Width = 1280
		c.Height = 960
	}

	if c.NumInspectFrames < 2 {
		c.NumInspectFrames = 2
	}

	if c.SenseThreshold == 0 {
		c.SenseThreshold = 4
	}

	if c.Protocol == "" || c.ListenPort == "" {
		c.Protocol = "tcp"
		c.ListenPort = ":9000"
	}

	c.getMaxBlockWidth()
}

func (c *Motion) publishAvg(caster *broker.Broker, frameAvg *[]motionVector, currFrame *[]motionVector) int {
	blocksTriggered := 0
	for i, v := range *frameAvg {
		if abs(v.X-(*currFrame)[i].X) > c.SenseThreshold || abs(v.Y-(*currFrame)[i].Y) > c.SenseThreshold {
			c.output[i] = 1
			blocksTriggered++
			//log.Printf("Frames: %2d Thresh: %2d Xavg: %3d X: %3d Yavg: %3d Y: %3d", c.NumInspectFrames, c.SenseThreshold, v.X, (*currFrame)[i].X, v.Y, (*currFrame)[i].Y)
			//log.Printf("Frames: %2d Thresh: %2d Xavg: %3d X: %3d", c.NumInspectFrames, c.SenseThreshold, v.X, (*currFrame)[i].X)
		} else {
			c.output[i] = 0
		}
	}

	//reportFrameAvgDiff(frameAvg, len(*frameAvg))
	//reportFrameAvgDiff(currFrame, len(*currFrame))
	caster.Publish(c.output)
	return blocksTriggered
}

func (c *Motion) publishHist(caster *broker.Broker, frameHistory *[][]motionVector) int {
	blocksTriggered := 0
	if len(*frameHistory) >= c.NumInspectFrames {
		for i := range (*frameHistory)[0] {
			c.output[i] = 0

			if len(c.MotionMask) > 0 && c.MotionMask[i] == 0 {
				continue
			}
			sxi := 0 // increasing X
			sxd := 0 // decreasing X
			syi := 0
			syd := 0
			for j := 0; j < c.NumInspectFrames-1; j++ {
				if (*frameHistory)[j][i].X < (*frameHistory)[j+1][i].X {
					sxi++
				}

				if (*frameHistory)[j][i].X > (*frameHistory)[j+1][i].X {
					sxd++
				}

				if (*frameHistory)[j][i].Y < (*frameHistory)[j+1][i].Y {
					syi++
				}

				if (*frameHistory)[j][i].Y > (*frameHistory)[j+1][i].Y {
					syd++
				}
			}

			if (sxi >= c.NumInspectFrames-1 ||
				sxd >= c.NumInspectFrames-1) &&
				(syi >= c.NumInspectFrames-1 ||
					syd >= c.NumInspectFrames-1) &&
				(abs((*frameHistory)[c.NumInspectFrames-2][i].X-(*frameHistory)[c.NumInspectFrames-1][i].X) > c.SenseThreshold ||
					abs((*frameHistory)[c.NumInspectFrames-2][i].Y-(*frameHistory)[c.NumInspectFrames-1][i].Y) > c.SenseThreshold) {
				c.output[i] = 1
				blocksTriggered++
				//log.Printf("Frames: %2d Caught: %2d X1: %3d X2: %3d X3: %3d", c.NumInspectFrames, fX, (*frameHistory)[0][i].X, (*frameHistory)[1][i].X, (*frameHistory)[2][i].X)
				//log.Printf("Frames: %2d Caught: %2d Y1: %3d Y2: %3d Y3: %3d", c.NumInspectFrames, fY, (*frameHistory)[0][i].Y, (*frameHistory)[1][i].Y, (*frameHistory)[2][i].Y)
				//log.Printf("Frames: %2d Caught: %2d X1: %3d X2: %3d X3: %3d X4: %3d", c.NumInspectFrames, fProcessed, (*frameHistory)[0][i].X, (*frameHistory)[1][i].X, (*frameHistory)[2][i].X, (*frameHistory)[3][i].X)
			}
			//log.Printf("Frames: %2d Thresh: %2d X1: %3d X2: %3d, X3: %3d, X4: %3d", c.NumInspectFrames, c.SenseThreshold, (*frameHistory)[0][i].X, (*frameHistory)[1][i].X, (*frameHistory)[2][i].X, v.X)
		}
	}

	//reportFrameAvgDiff(frameAvg, len(*frameAvg))
	//reportFrameAvgDiff(currFrame, len(*currFrame))
	caster.Publish(c.output)
	return blocksTriggered
}

// Detect motion by comparing the most recent frame with an average of the past numFrames.
// Lower senseThreshold value increases the sensitivity to motion.
func (c *Motion) Detect(caster *broker.Broker) {
	c.Init()
	conn := listen(c.Protocol, c.ListenPort)

	//f, _ := os.Create("motion.vec")
	numMacroblocks := ((c.Width + 16) / 16) * (c.Height / 16) // the right-most column is padding?
	numUsableMacroblocks := (c.Width / 16) * (c.Height / 16)

	currMacroBlocks := make([]motionVector, 0, numMacroblocks)
	currVectorBlocks := make([]motionVector, numUsableMacroblocks/(c.BlockWidth*c.BlockWidth))
	vectorAvgBlocks := make([]motionVector, numUsableMacroblocks/(c.BlockWidth*c.BlockWidth))
	_ = vectorAvgBlocks
	c.output = make([]byte, numUsableMacroblocks/(c.BlockWidth*c.BlockWidth))
	vectorHistory := make([][]motionVector, 0, c.NumInspectFrames)
	ignoredFrames := 0

	buf := make([]byte, 1024)
	s := bufio.NewReader(conn)
	blocksRead := 0
	for {
		_, err := s.Read(buf)

		if err != nil {
			log.Println("Motion detection stopped: " + err.Error())
			return
		}

		bufIdx := 0
		for bufIdx < len(buf) {
			// Manually convert since binary.Read runs really slow on a Pi Zero (~20% CPU)
			temp := motionVector{}
			temp.X = int8(buf[0+bufIdx])
			temp.Y = int8(buf[1+bufIdx])
			//temp.SAD = int16(buf[2+bufIdx]) << 4
			//temp.SAD |= int16(buf[3+bufIdx])
			currMacroBlocks = append(currMacroBlocks, temp)
			bufIdx += sizeofMotionVector
			blocksRead++

			if blocksRead == numMacroblocks {
				blocksRead = 0
				if ignoredFrames < ignoreFirstFrames {
					ignoredFrames++
					continue
				}
				c.condenseBlocksAvg(&currVectorBlocks, &currMacroBlocks)
				//c.findTemporalAverage(&vectorAvgBlocks, &vectorHistory, &currVectorBlocks)
				c.logVectorHistory(&vectorHistory, &currVectorBlocks)
				//if c.publishAvg(caster, &vectorAvgBlocks, &currVectorBlocks) > 0 {
				if c.publishHist(caster, &vectorHistory) > 0 {
					c.recorder.StopTime = time.Now().Add(time.Second * 2)
				}

				//binary.Write(f, binary.LittleEndian, &currMacroBlocks) // write to file
				currMacroBlocks = currMacroBlocks[:0]
			}
		}
	}
}

// Start motion detection and continues listening after interruptions to the data stream
func (c *Motion) Start(caster *broker.Broker, recorder *Recorder) {
	c.recorder = recorder
	for {
		c.Detect(caster)
	}
}
