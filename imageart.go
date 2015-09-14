package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	MaxRed   = 256
	MaxGreen = 256
	MaxBlue  = 256

	FullAlpha = 255

	MaxWidth  = 4096
	MaxHeight = 4096
)

var (
	seedImagePath     string
	seedImage         image.Image
	seedRejectionRate float64
	seedChroma        int
	seedDupes         bool

	echospacing float64

	seedColour int
	FirstRed   int32
	FirstGreen int32
	FirstBlue  int32
	p_red      int
	p_green    int
	p_blue     int

	colourAxes  string
	colourBasis ColourBasis

	SeedX int
	SeedY int

	LocalWidth  int
	LocalHeight int

	TargetRadius int32
	blur         int
	ChanSize     int32
	ch_cap       int
	cpu_cap      int

	DrawIntermediate bool
	FlipDraw         bool

	PicTag  string
	PicName string
)

func parseFlags() {
	flag.IntVar(&p_red, "seed-red", 0, "red value of the initial point")
	flag.IntVar(&p_green, "seed-green", 0, "green value of the initial point")
	flag.IntVar(&p_blue, "seed-blue", 0, "blue value of the initial point")

	flag.StringVar(&colourAxes, "colour-basis", "rgb", "colour axes to use: one of [rgb, rbg, gbr, grb, bgr, brg]")

	flag.IntVar(&LocalWidth, "width", 4096, "Output image width (if not using seed image)")
	flag.IntVar(&LocalHeight, "height", 4096, "Output image height (if not using seed image")

	flag.IntVar(&seedColour, "seed", 0x0, "seed colour (e.g. 0xFFFFFF)")
	flag.IntVar(&SeedX, "seed-x", 0, "x position of the initial point")
	flag.IntVar(&SeedY, "seed-y", 0, "y position of the initial point")
	flag.StringVar(&seedImagePath, "seed-image", "", "Pre-seeded image to fill. Empty pixels are 0x000000")
	flag.Float64Var(&seedRejectionRate, "seed-rr", 0, "Random rejection rate of seeded pixels between 0 and 1")
	flag.IntVar(&seedChroma, "seed-chroma-key", 0xFF00FF, "Colour to treat as empty in seeded image")
	flag.BoolVar(&seedDupes, "seed-dupes", false, "Search for repeated colours in input image. Takes a bit.")

	flag.IntVar(&blur, "blur", 1, "higher values increase time required to complete image.")

	flag.IntVar(&ch_cap, "chan", 8, "very high values produce geometric patterns originating about the initial point.")

	flag.StringVar(&PicName, "name", "", "name to use for final image file")
	flag.StringVar(&PicTag, "tag", "art", "tags for intermediate representation and final file (if no PicName specified)")

	flag.BoolVar(&DrawIntermediate, "ir", false, "draw intermediate representations of the image")
	flag.BoolVar(&FlipDraw, "flip-draw", false, "flip ALL colours at the bit level after running")

	flag.Float64Var(&echospacing, "es", 0, "Turn on echospacing/reseeding")

	flag.IntVar(&cpu_cap, "cpus", -1, "amount of cpu's used. 0 means default go runtime settings, <0 means 'use all' (default)")

	flag.Parse()

	if seedColour != 0x0 {
		p_red = seedColour >> 16
		p_green = (seedColour & 0x00FF00) >> 8
		p_blue = seedColour & 0x0000FF
	}

	if seedImagePath != "" {
		extension := filepath.Ext(seedImagePath)

		file, err := os.Open(seedImagePath)

		if err != nil {
			panic(err)
		}

		if extension == ".png" {
			seedImage, err = png.Decode(file)
		} else if extension == ".jpeg" || extension == ".jpg" {
			seedImage, err = jpeg.Decode(file)
		} else {
			fmt.Println("Cannot open file", seedImagePath)
		}

		if err != nil {
			panic(err)
		}
		err = file.Close()
		if err != nil {
			panic(err)
		}

		LocalWidth = seedImage.Bounds().Max.X
		LocalHeight = seedImage.Bounds().Max.Y
	}

	switch colourAxes {
	case "rgb":
		colourBasis = RGB
	case "rbg":
		colourBasis = RBG
	case "gbr":
		colourBasis = GBR
	case "grb":
		colourBasis = GRB
	case "bgr":
		colourBasis = BGR
	case "brg":
		colourBasis = BRG
	}
}

///// math functions

func sqr(x int32) int32 {
	return x * x
}

func distSqr(a, b, c, x, y, z int32) (d int32) {
	d = (a-x)*(a-x) + (b-y)*(b-y) + (c-z)*(c-z)
	return
}

func maxint32(a, b int32) int32 {
	if b > a {
		return b
	}
	return a
}

func maxint(a, b int) int {
	if b > a {
		return b
	}
	return a
}

func minint(a, b int) int {
	if b < a {
		return b
	}
	return a
}

func timestamp() string {
	var hour, min, sec = time.Now().Clock()
	return fmt.Sprintf("%02d:%02d:%02d", hour, min, sec)
}

///// the rest of the code and whatnot

func fillPixelArray(pArray *PixelArray, cspace Colourspace, seedCh chan SeedPixel, ch chan image.Point) (count int32) {
	// for printing intermediate images
	origPicName := PicName
	var ir_tag int32 = 1
	var tenth_of_pic int32 = int32(LocalWidth*LocalHeight) / 10
	var tmp_colour Colour24

	var seeds int32 = 0
	//put in the seed pixels
	rand.Seed(time.Now().UnixNano())
	for {
		sp, more := <-seedCh
		if more {
			// seed pixel from channel

			if !cspace.ColourUsed(sp) || seedDupes {
				seeds++
				tmp_colour = cspace.PopColour(sp)

				pArray.Set(int32(sp.Pt.X), int32(sp.Pt.Y), tmp_colour)

				go func() {
					for x_offset := -1; x_offset < 2; x_offset++ {
						if sp.Pt.X+x_offset < LocalWidth && sp.Pt.X+x_offset >= 0 {
							for y_offset := -1; y_offset < 2; y_offset++ {
								if sp.Pt.Y+y_offset < LocalHeight && sp.Pt.Y+y_offset >= 0 && !(x_offset == 0 && y_offset == 0) {
									pt := image.Pt(sp.Pt.X+x_offset, sp.Pt.Y+y_offset)
									if !pArray.QueuedAt(int32(pt.X), int32(pt.Y)) && !pArray.FilledAt(int32(pt.X), int32(pt.Y)) {
										pArray[pt.X][pt.Y].Queued = true
										ch <- pt
									}
								}
							}
						}
					}
				}()
			}

		} else {
			// all seeds have been recieved
			fmt.Println(seeds, "seeded pixels")
			break
		}
	}

	for count = seeds; count < int32(LocalWidth*LocalHeight); count++ {
		point := <-ch

		//in the case of a point being re-queued after being filled
		//TODO: assert !FilledAt
		if pArray.FilledAt(int32(point.X), int32(point.Y)) {
			count--
			continue
		}

		tmp_colour = pArray.TargetColourAt(int32(point.X), int32(point.Y))

		tmp_colour = cspace.PopColour(tmp_colour)

		// it's nice to know the algorithm is running
		if count > ir_tag*tenth_of_pic && ir_tag < 10 {
			fmt.Printf("[%s] %d%% of pixels filled\n", timestamp(), ir_tag*10)
			if DrawIntermediate {
				PicName = fmt.Sprintf("%s.%d.png", PicTag, ir_tag)
				go draw(pArray.ImageNRGBA())
			}
			ir_tag++
		}

		if count == MaxWidth*MaxHeight*15/16 {
			fmt.Println("Endgame optimisation... (this last one takes the longest :( )")
			cspace.PrepOpt()
		}

		pArray.Set(int32(point.X), int32(point.Y), tmp_colour)

		go func() {
			for x_offset := -1; x_offset < 2; x_offset++ {
				if point.X+x_offset < LocalWidth && point.X+x_offset >= 0 {
					for y_offset := -1; y_offset < 2; y_offset++ {
						if point.Y+y_offset < LocalHeight && point.Y+y_offset >= 0 && !(x_offset == 0 && y_offset == 0) {
							pt := image.Pt(point.X+x_offset, point.Y+y_offset)
							if !pArray.QueuedAt(int32(pt.X), int32(pt.Y)) && !pArray.FilledAt(int32(pt.X), int32(pt.Y)) {
								pArray[pt.X][pt.Y].Queued = true
								ch <- pt
							}
						}
					}
				}
			}
		}()
	}

	PicName = origPicName
	return
}

func processSeedImage(seedCh chan SeedPixel) {
	bounds := seedImage.Bounds()
	// spiral seeding

	checkAndSeed := func(x, y int) { // if pixel != chroma, add to seed queue
		var pixel SeedPixel
		r, g, b, _ := seedImage.At(x, y).RGBA()
		if (r/256<<16)|(g/256<<8)|b/256 != uint32(seedChroma) {
			// randomly cull a set percentage to get more balanced images
			if seedRejectionRate > 0 {
				if rand.Float64() > seedRejectionRate {
					if FlipDraw {
						pixel = NewSeedPixel(uint8(255-r), uint8(255-g), uint8(255-b), x, y)
					} else {
						pixel = NewSeedPixel(uint8(r), uint8(g), uint8(b), x, y)
					}

					seedCh <- pixel
				}
			}

		}
	}

	// iterate down the given line
	scanLine := func(startX, endX, startY, endY int) {
		if startX == endX {
			x := startX
			if startY < endY {
				for y := startY; y < endY; y++ {
					checkAndSeed(x, y)
				}
			} else {
				for y := startY; y > endY; y-- {
					checkAndSeed(x, y)
				}
			}
		} else {
			y := startY
			if startX < endX {
				for x := startX; x < endX; x++ {
					checkAndSeed(x, y)
				}
			} else {
				for x := startX; x > endX; x-- {
					checkAndSeed(x, y)
				}
			}
		}
	}

	// seed first pixel
	checkAndSeed(SeedX, SeedY)

	var x, y int
	var layer int = 0
	var xMinCap, xMaxCap, yMinCap, yMaxCap bool = false, false, false, false
	var start, end int

	// while not at all four boundaries, scan the image
	for !xMinCap || !xMaxCap || !yMinCap || !yMaxCap {
		layer++

		// topright to topleft
		y = SeedY - layer
		if y < bounds.Min.Y {
			yMinCap = true
		} else {
			start = minint(SeedX+layer, bounds.Max.X-1)
			end = maxint(SeedX-layer, bounds.Min.X)
			scanLine(start, end, y, y)
		}

		//topleft to botleft
		x = SeedX - layer
		if x < bounds.Min.X {
			xMinCap = true
		} else {
			start = maxint(SeedY-layer, bounds.Min.Y)
			end = minint(SeedY+layer, bounds.Max.Y-1)
			scanLine(x, x, start, end)
		}

		// botleft to botright
		y = SeedY + layer
		if y >= bounds.Max.Y {
			yMaxCap = true
		} else {
			start = maxint(SeedX-layer, bounds.Min.X)
			end = minint(SeedX+layer, bounds.Max.X-1)
			scanLine(start, end, y, y)
		}

		//botright to topright
		x = SeedX + layer
		if x >= bounds.Max.X {
			xMaxCap = true
		} else {
			start = minint(SeedY+layer, bounds.Max.Y-1)
			end = maxint(SeedY-layer, 0)
			scanLine(x, x, start, end)
		}
	}
	close(seedCh)
}

func composeImageName() (name string) {
	name = fmt.Sprintf("%s.%s", PicTag, colourAxes)

	if seedImage == nil {
		name += fmt.Sprintf(".r%dg%db%d", FirstRed, FirstGreen, FirstBlue)
	} else {
		name += fmt.Sprintf(".rr%1.3f", seedRejectionRate)
	}

	name += fmt.Sprintf(".x%dy%d.blur%d.ch%d.cpu%d", SeedX, SeedY, TargetRadius, ChanSize, runtime.GOMAXPROCS(0))

	if FlipDraw {
		name += ".flip"
	}

	if echospacing > 0 {
		name += fmt.Sprintf(".es%1.5f", echospacing)
	}

	name += ".png"
	return
}

func main() {

	var start_hour, start_min, start_sec = time.Now().Clock()
	fmt.Printf("Start time: %d:%d:%d\n", start_hour, start_min, start_sec)

	parseFlags()

	// Also affects CPU scheduling I suppose :)
	if cpu_cap != 0 {
		if (cpu_cap < 0) || (cpu_cap > runtime.NumCPU()) {
			cpu_cap = runtime.NumCPU()
		}
		runtime.GOMAXPROCS(cpu_cap)
		cpu_cap = runtime.GOMAXPROCS(0)
	}
	if cpu_cap > 1 {
		fmt.Printf("Using %d CPUs\n", cpu_cap)
	} else {
		fmt.Printf("Using %d CPU\n", cpu_cap)
	}

	var seedCh chan SeedPixel

	var colours Colourspace = GetColourspace(colourBasis)
	colours.SetEchospace(echospacing)

	picture := new(PixelArray)

	if seedImage != nil {
		bounds := seedImage.Bounds()
		chanSize := (bounds.Max.X * bounds.Max.Y)
		if seedRejectionRate > 0 {
			chanSize = int(float64(chanSize) * (1 - (seedRejectionRate * seedRejectionRate)))
		}
		seedCh = make(chan SeedPixel, chanSize)
		go processSeedImage(seedCh)

	} else {
		// seeding based on params rather than seed image
		seedCh = make(chan SeedPixel, 1)
		seedCh <- NewSeedPixel(uint8(p_red), uint8(p_green), uint8(p_blue), SeedX, SeedY)

		close(seedCh)
	}

	TargetRadius = int32(blur)
	ChanSize = int32(ch_cap)
	if PicName == "" {
		PicName = composeImageName()
	}

	// changing channel size affects behaviour of colour filling;
	// or rather, it makes CPU scheduling choices have a greater impact
	ch := make(chan image.Point, ChanSize)
	_ = fillPixelArray(picture, colours, seedCh, ch)

	draw(picture.ImageNRGBA())

	var end_hour, end_min, end_sec = time.Now().Clock()
	fmt.Printf("End time: %d:%d:%d\n", end_hour, end_min, end_sec)
	end_sec -= start_sec
	if end_sec < 0 {
		end_sec = 60 + end_sec
		end_min -= 1
	}
	end_min -= start_min
	if end_min < 0 {
		end_min = 60 + end_min
		end_hour -= 1
	}
	end_hour -= start_hour
	fmt.Printf("Image drawn in %d:%d:%d\n", end_hour, end_min, end_sec)

}
