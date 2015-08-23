package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
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

	flag.IntVar(&LocalWidth, "width", MaxWidth, "Output image width (if not using seed image)")
	flag.IntVar(&LocalHeight, "height", MaxHeight, "Output image height (if not using seed image")

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

	flag.IntVar(&cpu_cap, "cpus", 0, "amount of cpu's used. 0 means default go runtime settings, <0 means 'use all' (default)")

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

func maxint(a, b int32) int32 {
	if b > a {
		return b
	}
	return a
}

func timestamp() string {
	var hour, min, sec = time.Now().Clock()
	return fmt.Sprintf("%02d:%02d:%02d", hour, min, sec)
}

///// type declarations

// Pixel meta-struct
type colour24 struct {
	red   uint8
	green uint8
	blue  uint8
}

type SeedPixel struct {
	Colour colour24
	Pt     image.Point
}

func NewSeedPixel(r, g, b uint8, x, y int) (spixel SeedPixel) {
	spixel.Colour = colour24{r, g, b}
	spixel.Pt = image.Point{x, y}
	return
}

func (c *SeedPixel) Red() int32 {
	return int32(c.Colour.red)
}

func (c *SeedPixel) Green() int32 {
	return int32(c.Colour.green)
}

func (c *SeedPixel) Blue() int32 {
	return int32(c.Colour.blue)
}

type Pixel struct {
	Colour colour24
	Filled bool
	Queued bool
}

func (c *Pixel) Red() int32 {
	return int32(c.Colour.red)
}

func (c *Pixel) Green() int32 {
	return int32(c.Colour.green)
}

func (c *Pixel) Blue() int32 {
	return int32(c.Colour.blue)
}

func (c *Pixel) NRGBA() *color.NRGBA {
	if FlipDraw {
		return &color.NRGBA{255 - c.Colour.red, 255 - c.Colour.green, 255 - c.Colour.blue, FullAlpha}
	} else {
		return &color.NRGBA{c.Colour.red, c.Colour.green, c.Colour.blue, FullAlpha}
	}
}

type PixelArray [MaxWidth][MaxHeight]Pixel

func (c *PixelArray) ImageNRGBA() *image.NRGBA {
	pic := image.NewNRGBA(image.Rect(0, 0, LocalWidth, LocalHeight))
	for x := 0; x < LocalWidth; x++ {
		for y := 0; y < LocalHeight; y++ {
			pic.Set(x, y, c[x][y].NRGBA())
		}
	}
	return pic
}

func (c *PixelArray) Set(x, y, red, green, blue int32) {
	c[x][y].Colour.red = uint8(red)
	c[x][y].Colour.green = uint8(green)
	c[x][y].Colour.blue = uint8(blue)
	c[x][y].Queued = true
	c[x][y].Filled = true
}

func (c *PixelArray) ColourAt(x, y int32) (red, green, blue int32) {
	red = c[x][y].Red()
	green = c[x][y].Green()
	blue = c[x][y].Blue()
	return
}

func (c *PixelArray) FilledAt(x, y int32) bool {
	return c[x][y].Filled
}

func (c *PixelArray) QueuedAt(x, y int32) bool {
	return c[x][y].Queued
}

// TargetColourAt examines the neighbouring pixels to the point (x, y) and
// returns an unweighted average of the red, green, and blue channels of
// the filled nearby pixels.
func (c *PixelArray) TargetColourAt(x, y int32) (red, green, blue int32) {

	var r_sum, g_sum, b_sum float64 // temps to hold the colours of surrounding pixels
	var filled_neighbours float64
	var x_offset, y_offset int32

	for x_offset = -TargetRadius; x_offset <= TargetRadius; x_offset++ {
		if x+x_offset < int32(LocalWidth) && x+x_offset >= 0 {
			for y_offset = -TargetRadius; y_offset <= TargetRadius; y_offset++ {

				if x_offset == 0 && y_offset == 0 {
					//skip calling pixel
					continue
				}

				if y+y_offset < int32(LocalHeight) && y+y_offset >= 0 {
					if c.FilledAt(x+x_offset, y+y_offset) {
						r, g, b := c.ColourAt(x+x_offset, y+y_offset)
						r_sum += float64(r)
						g_sum += float64(g)
						b_sum += float64(b)
						filled_neighbours++
					}
				}
			}
		}
	}

	if filled_neighbours > 0 {
		r_sum /= filled_neighbours
		g_sum /= filled_neighbours
		b_sum /= filled_neighbours
		red = int32(math.Floor(r_sum + 0.5))
		green = int32(math.Floor(g_sum + 0.5))
		blue = int32(math.Floor(b_sum + 0.5))
	}
	return
}

///// the rest of the code and whatnot

func draw(pic *image.NRGBA) {
	fmt.Println("Drawing", PicName)
	file, err := os.Create(PicName)

	if err != nil {
		panic(err)
	}
	err = png.Encode(file, pic)
	if err != nil {
		panic(err)
	}
	err = file.Close()
	if err != nil {
		panic(err)
	}
}

func fillPixelArray(pArray *PixelArray, cspace Colourspace, seedCh chan SeedPixel, ch chan image.Point) (count int32) {
	// for printing intermediate images
	origPicName := PicName
	var ir_tag int32 = 1
	var tenth_of_pic int32 = int32(LocalWidth*LocalHeight) / 10
	var red, green, blue int32

	// initial pop function
	GetNearestColour := cspace.PopColour

	var seeds int32 = 0
	//put in the seed pixels
	rand.Seed(time.Now().UnixNano())
	for {
		sp, more := <-seedCh
		if more {
			// seed pixel from channel

			// randomly cull a set percentage to get more balanced images
			if seedRejectionRate > 0 {
				if rand.Float64() < seedRejectionRate {
					continue
				}
			}

			if !cspace.ColourUsed(sp.Red(), sp.Green(), sp.Blue()) || seedDupes {
				seeds++
				red, green, blue = GetNearestColour(sp.Red(), sp.Green(), sp.Blue())

				pArray.Set(int32(sp.Pt.X), int32(sp.Pt.Y), red, green, blue)

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

		red, green, blue = pArray.TargetColourAt(int32(point.X), int32(point.Y))

		red, green, blue = GetNearestColour(red, green, blue)

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
			cspace.PrepCounts()
			GetNearestColour = cspace.PopColourOpt
		}

		pArray.Set(int32(point.X), int32(point.Y), red, green, blue)

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
			if FlipDraw {
				pixel = NewSeedPixel(uint8(255-r), uint8(255-g), uint8(255-b), x, y)
			} else {
				pixel = NewSeedPixel(uint8(r), uint8(g), uint8(b), x, y)
			}
			seedCh <- pixel
		}
	}

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

	for !xMinCap || !xMaxCap || !yMinCap || !yMaxCap {
		layer++

		// topright to topleft
		y = SeedY - layer
		if y < bounds.Min.Y {
			yMinCap = true
		} else {

			if SeedX-layer < 0 {
				end = 0
			} else {
				end = SeedX - layer
			}
			if SeedX+layer >= bounds.Max.X {
				start = bounds.Max.X - 1
			} else {
				start = SeedX + layer
			}
			scanLine(start, end, y, y)
		}

		//topleft to botleft
		x = SeedX - layer
		if x < bounds.Min.X {
			xMinCap = true
		} else {

			if SeedY-layer < 0 {
				start = 0
			} else {
				start = SeedY - layer
			}
			if SeedY+layer >= bounds.Max.Y {
				end = bounds.Max.Y - 1
			} else {
				end = SeedY + layer
			}
			scanLine(x, x, start, end)
		}

		// botleft to botright
		y = SeedY + layer
		if y >= bounds.Max.Y {
			yMaxCap = true
		} else {

			if SeedX-layer < 0 {
				start = 0
			} else {
				start = SeedX - layer
			}
			if SeedX+layer >= bounds.Max.X {
				end = bounds.Max.X - 1
			} else {
				end = SeedX + layer
			}
			scanLine(start, end, y, y)
		}

		//botright to topright
		x = SeedX + layer
		if x >= bounds.Max.X {
			xMaxCap = true
		} else {

			if SeedY-layer < 0 {
				end = 0
			} else {
				end = SeedY - layer
			}
			if SeedY+layer >= bounds.Max.Y {
				start = bounds.Max.Y - 1
			} else {
				start = SeedY + layer
			}
			scanLine(x, x, start, end)
		}
	}
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

	picture := new(PixelArray)

	if seedImage != nil {
		bounds := seedImage.Bounds()
		seedCh = make(chan SeedPixel, bounds.Max.X*bounds.Max.Y)
		processSeedImage(seedCh)

	} else {
		// seeding based on params rather than seed image
		seedCh = make(chan SeedPixel, 1)
		seedCh <- NewSeedPixel(uint8(p_red), uint8(p_green), uint8(p_blue), SeedX, SeedY)
	}

	close(seedCh)

	TargetRadius = int32(blur)
	ChanSize = int32(ch_cap)
	if PicName == "" {
		flipped := ""
		if FlipDraw {
			flipped = ".flip"
		}
		if seedImage == nil {
			PicName = fmt.Sprintf("%s.%s.r%dg%db%d.x%dy%d.blur%d.ch%d.cpu%d%s.png", PicTag, colourAxes, FirstRed, FirstGreen, FirstBlue, SeedX, SeedY, TargetRadius, ChanSize, runtime.GOMAXPROCS(0), flipped)
		} else {
			PicName = fmt.Sprintf("%s.%s.rr%1.3f.x%dy%d.blur%d.ch%d.cpu%d%s.png", PicTag, colourAxes, seedRejectionRate, SeedX, SeedY, TargetRadius, ChanSize, runtime.GOMAXPROCS(0), flipped)
		}
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
