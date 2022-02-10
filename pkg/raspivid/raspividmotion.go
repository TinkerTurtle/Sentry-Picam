package raspivid

/*
Usage:
	cam := Motion{}

	castMotion := broker.New()
	go castMotion.Start()
	go cam.Start(castMotion)

	reader := castMotion.Subscribe()
	defer castMotion.Unsubscribe(reader)
	for {
		fmt.Println(<-reader)
	}

*/
import (
	"bufio"
	"log"
	"os"
	"time"

	"sentry-picam/broker"
)

const ignoreFirstFrames = 10 // give camera's autoexposure some time to settle
// Motion stores configuration parameters and forms the basis for Detect
type Motion struct {
	Width           int
	Height          int
	SenseThreshold  int8
	BlockWidth      int
	Protocol        string
	ListenPort      string
	MotionMask      []byte
	output          []byte
	recorder        *Recorder
	RecordingFolder string

	rowCount       int
	colCount       int
	mColCount      int
	mCenterX       int
	mCenterY       int
	usableCols     int
	highlightDistX int
	highlightDistY int
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
	tX, tY, tXn, tYn int8 // counters for increasing and decreasing X/Y vectors
	count            int
}

func (mV *mVhelper) add(v motionVector) {
	mV.count++

	if v.X > 0 {
		mV.tX++
	} else if v.X < 0 {
		mV.tXn++
	}
	if v.Y > 0 {
		mV.tY++
	} else if v.Y < 0 {
		mV.tYn++
	}
}

// getAvg figures out if the motion vectors are in the same general direction
func (mV *mVhelper) getAvg(threshold int8) motionVector {
	if (mV.tX >= threshold || mV.tXn >= threshold) &&
		(mV.tY >= threshold || mV.tYn >= threshold) {

		return motionVector{
			1,
			1,
			//int16(mV.SAD),
		}
	}
	return motionVector{
		0,
		0,
		//0,
	}
}

func (mV *mVhelper) reset() {
	mV.count = 0
	mV.tX = 0
	mV.tXn = 0
	mV.tY = 0
	mV.tYn = 0
}

// ApplyMask applies a mask to ignore specified motion blocks
func (c *Motion) ApplyMask(mask []byte) {
	c.MotionMask = mask

	f, _ := os.Create(c.RecordingFolder + "motionMask.bin")
	f.Write(mask)
	defer f.Close()
}

// ApplyPreviousMask applies the previously registered motion mask
func (c *Motion) ApplyPreviousMask() {
	f, err := os.ReadFile(c.RecordingFolder + "motionMask.bin")
	if err != nil {
		log.Printf("Couldn't load motion mask")
		return
	}

	rowCount := c.Height / 16
	colCount := (c.Width + 16) / 16
	usableCols := colCount - 1

	maskLength := (usableCols / c.BlockWidth) * (rowCount / c.BlockWidth)
	if len(f) == maskLength {
		c.MotionMask = f
		log.Printf("Previously registered motion mask has been applied")
	} else {
		log.Printf("Couldn't apply previous motion mask due to changed resolution")
	}
}

// condenseBlocksDirection takes a blockWidth * blockWidth average of macroblocks from buf and stores the
// condensed result into frame
func (c *Motion) condenseBlocksDirection(frame *[]motionVector, buf *[]motionVector) {
	mV := make([]mVhelper, c.usableCols/c.BlockWidth)
	i := 0
	compressedIndex := 0

	for x := 0; x < c.rowCount; x++ {
		blk := 1
		blkIndex := 0
		for y := 0; y < c.colCount; y++ {
			if y < c.usableCols {
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
				if len(c.MotionMask) > 0 && c.MotionMask[compressedIndex] == 0 {
					(*frame)[compressedIndex] = motionVector{0, 0}
				} else {
					(*frame)[compressedIndex] = v.getAvg(c.SenseThreshold)
				}
				mV[idx].reset()
				compressedIndex++
			}
		}
	}
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
func (c *Motion) Init(usePreviousMask bool) {
	if c.Width == 0 || c.Height == 0 {
		c.Width = 1280
		c.Height = 960
	}

	if c.SenseThreshold == 0 {
		c.SenseThreshold = 9
	}

	if c.Protocol == "" || c.ListenPort == "" {
		c.Protocol = "tcp"
		c.ListenPort = ":9000"
	}

	c.getMaxBlockWidth()
	if c.SenseThreshold > int8(c.BlockWidth*c.BlockWidth) {
		c.SenseThreshold = int8(c.BlockWidth)
		log.Printf("mthreshold lowered to %d\n", c.SenseThreshold)
	}
	log.Printf("Motion threshold: %d / %d\n", c.SenseThreshold, c.BlockWidth*c.BlockWidth)

	if usePreviousMask {
		c.ApplyPreviousMask()
	}
}

func (c *Motion) publishParsedBlocks(caster *broker.Broker, frame *[]motionVector) int {
	blocksTriggered := 0
	for i, v := range *frame {
		c.output[i] = 0
		if v.X != 0 {
			//log.Printf("Frames: %2d X1: %3d Y1: %3d", c.NumInspectFrames, v.X, v.Y)
			c.output[i] = 1
			blocksTriggered++
		}
	}

	caster.Publish(c.output)
	return blocksTriggered
}

func abs(x int) int {
	if x > 0 {
		return x
	}
	return -x
}

func (c *Motion) checkHighlight(frame *[]motionVector) {
	for i, v := range *frame {
		currDistX := abs(i%c.mColCount - c.mCenterX)
		currDistY := abs(i/c.mColCount - c.mCenterY)
		if v.X != 0 && c.highlightDistX >= currDistX && c.highlightDistY >= currDistY {
			c.highlightDistX = currDistX
			c.highlightDistY = currDistY
			c.recorder.HighlightTime = time.Now()
		}
	}
}

// triggeredEdge checks if any sectors with motion are not on the edge of the frame
func (c *Motion) triggeredNonEdge(frame *[]motionVector) bool {
	motionWidth := c.Width / c.BlockWidth
	for i, v := range *frame {
		if v.X != 0 {
			if i <= motionWidth { // top of frame
				continue
			} else if i > len(*frame)-motionWidth { // bottom of frame
				continue
			} else if i%motionWidth == 0 { // left of frame
				continue
			} else if (i+1)%motionWidth == 0 { // right of frame
				continue
			} else {
				return true
			}
		}
	}
	return false
}

// Detect parses motion vectors provided by raspivid
// Lower senseThreshold value increases the sensitivity to motion.
func (c *Motion) Detect(caster *broker.Broker) {
	conn := listen(c.Protocol, c.ListenPort)

	//f, _ := os.Create("motion.vec")
	numMacroblocks := ((c.Width + 16) / 16) * (c.Height / 16) // the right-most column is padding?
	numUsableMacroblocks := (c.Width / 16) * (c.Height / 16)

	currMacroBlocks := make([]motionVector, 0, numMacroblocks)
	currCondensedBlocks := make([]motionVector, numUsableMacroblocks/(c.BlockWidth*c.BlockWidth))
	c.output = make([]byte, numUsableMacroblocks/(c.BlockWidth*c.BlockWidth))

	c.rowCount = c.Height / 16
	c.colCount = (c.Width + 16) / 16
	c.usableCols = c.colCount - 1
	c.mColCount = c.usableCols / c.BlockWidth
	c.mCenterX = c.rowCount / c.BlockWidth / 2
	c.mCenterY = c.colCount / c.BlockWidth / 2

	c.highlightDistX = c.rowCount
	c.highlightDistY = c.colCount

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
			//temp.SAD = int16(buf[2+bufIdx]) << 4 // SAD might be spiking around keyframes and triggers false positives
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
				c.condenseBlocksDirection(&currCondensedBlocks, &currMacroBlocks)
				if c.publishParsedBlocks(caster, &currCondensedBlocks) > 0 {
					c.checkHighlight(&currCondensedBlocks)
					if time.Now().After(c.recorder.StopTime) {
						// reset highlight distance
						c.highlightDistX = c.rowCount
						c.highlightDistY = c.colCount
					}
					if c.triggeredNonEdge(&currCondensedBlocks) {
						c.recorder.StopTime = time.Now().Add(time.Second * 10)
					} else {
						c.recorder.StopTime = time.Now().Add(time.Second * 5)
					}
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
