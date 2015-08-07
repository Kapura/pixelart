package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
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
	FirstRed   int32
	FirstGreen int32
	FirstBlue  int32
	p_red      int
	p_green    int
	p_blue     int

	StartX int
	StartY int

	TargetRadius int32
	blur         int
	ChanSize     int32
	ch_cap       int
	cpu_cap      int

	PicName string
)

func parseFlags() {
	flag.IntVar(&p_red, "seed-red", 0, "red value of the initial point")
	flag.IntVar(&p_green, "seed-green", 0, "green value of the initial point")
	flag.IntVar(&p_blue, "seed-blue", 0, "blue value of the initial point")

	flag.IntVar(&StartX, "seed-x", 0, "x position of the initial point")
	flag.IntVar(&StartY, "seed-y", 0, "y position of the initial point")

	flag.IntVar(&blur, "blur", 1, "higher values increase time required to complete image.")

	flag.IntVar(&ch_cap, "chan", 8, "very high values produce geometric patterns originating about the initial point.")

	flag.StringVar(&PicName, "name", "", "name to use for final image file")

	flag.IntVar(&cpu_cap, "cpus", 0, "amount of cpu's used. 0 means default go runtime settings, <0 means 'use all' (default)")
	flag.Parse()
}

///// math functions

func sqr(x int32) int32 {
	return x * x
}

func distSqr(a, b, c, x, y, z int32) (d float64) {
	d = float64(sqr(a-x) + sqr(b-y) + sqr(c-z))
	return
}

func maxint(a, b int32) int32 {
	if b > a {
		return b
	}
	return a
}

///// type declarations

// RGBCube represents png colour values in 3space
// field becomes true when a colour is placed in the image.
type RGBCube [MaxRed][MaxGreen][MaxBlue]bool

// Pixel meta-struct
type Pixel struct {
	Colour struct {
		red   uint8
		green uint8
		blue  uint8
	}
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
	return &color.NRGBA{c.Colour.red, c.Colour.green, c.Colour.blue, FullAlpha}
}

type PixelArray [MaxWidth][MaxHeight]Pixel

func (c *PixelArray) ImageNRGBA() *image.NRGBA {
	pic := image.NewNRGBA(image.Rect(0, 0, MaxWidth, MaxHeight))
	for x := 0; x < MaxWidth; x++ {
		for y := 0; y < MaxHeight; y++ {
			pic.Set(x, y, c[x][y].NRGBA())
		}
	}
	return pic
}

func (c *PixelArray) Set(x, y, red, green, blue int32) {
	c[x][y].Colour.red = uint8(red)
	c[x][y].Colour.green = uint8(green)
	c[x][y].Colour.blue = uint8(blue)
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
		if x+x_offset < MaxWidth && x+x_offset >= 0 {
			for y_offset = -TargetRadius; y_offset <= TargetRadius; y_offset++ {

				if x_offset == 0 && y_offset == 0 {
					//skip calling pixel
					continue
				}

				if y+y_offset < MaxHeight && y+y_offset >= 0 {
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
	fmt.Println("Drawing...")
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

func fillPixelArray(pArray *PixelArray, cube *RGBCube, ch chan image.Point) (count int32) {
	for count = 0; count < MaxWidth*MaxHeight; count++ {
		point := <-ch

		if pArray.FilledAt(int32(point.X), int32(point.Y)) {
			count--
			continue
		}

		var red, green, blue int32
		if count == 0 {
			red = FirstRed
			green = FirstGreen
			blue = FirstBlue
		} else {
			red, green, blue = pArray.TargetColourAt(int32(point.X), int32(point.Y))
		}

		red, green, blue = nearestAvailableColour(red, green, blue, cube)

		// it's nice to know the algorithm is running
		if count == MaxWidth*MaxHeight*1/2 {
			fmt.Println("1/2")
		}

		if count == MaxWidth*MaxHeight*3/4 {
			fmt.Println("3/4")
		}

		if count == MaxWidth*MaxHeight*7/8 {
			fmt.Println("7/8")
		}

		if count == MaxWidth*MaxHeight*15/16 {
			fmt.Println("15/16 (this last one takes the longest :( )")
		}

		pArray.Set(int32(point.X), int32(point.Y), red, green, blue)

		go repopulateChannel(ch, point, pArray)
	}
	return
}

func repopulateChannel(ch chan image.Point, point image.Point, pArray *PixelArray) {
	for x_offset := -1; x_offset < 2; x_offset++ {
		if point.X+x_offset < MaxWidth && point.X+x_offset >= 0 {
			for y_offset := -1; y_offset < 2; y_offset++ {
				if point.Y+y_offset < MaxHeight && point.Y+y_offset >= 0 && !(x_offset == 0 && y_offset == 0) {
					pt := image.Pt(point.X+x_offset, point.Y+y_offset)
					if !pArray.QueuedAt(int32(pt.X), int32(pt.Y)) && !pArray.FilledAt(int32(pt.X), int32(pt.Y)) {
						pArray[pt.X][pt.Y].Queued = true
						ch <- pt
					}
				}
			}
		}
	}
}

func nearestAvailableColour(r, g, b int32, colours *RGBCube) (red, green, blue int32) {
	// check the colour cube to see if the colour has been painted yet
	if !colours[r][g][b] {
		// false = we can use it
		red = r
		green = g
		blue = b
	} else {
		// true = colour already used, need to find closest unused colour
		// search colour 3space in increasing spherical shells
		outer_radius := int32(0)                 // radius of the search sphere
		min_dist_sqr := float64(math.MaxFloat64) // initialised farther than largest possible radius
		found := false
		for !found && outer_radius < MaxRed {
			outer_radius++
			previous_shell_sqr := math.Pow(float64(outer_radius-1), 2)
			// expand in sphere about src pt until a colour is false (that is to say, unused)
			for i := maxint(0, r-outer_radius); i < r+outer_radius && i < MaxRed; i++ {
				// i traverses the end to end height of the sphere one level at a time i ~ r
				inner_radius := math.Sqrt(float64(sqr(outer_radius) + sqr(i-r)))
				// inner_radius = radius of (green-blue) circle at level i along sphere
				for j := maxint(0, g-int32(inner_radius)); j < g+int32(inner_radius) && j < MaxGreen; j++ {
					// j traverses the end to end width of the gb circle along the g axis one row at a time j ~ g
					segment_level := math.Sqrt(float64(sqr(int32(inner_radius)) + sqr(j-g)))
					for k := maxint(0, b-int32(segment_level)); k < b+int32(segment_level) && k < MaxBlue; k++ { // k ~ b
						this_dist_sqr := distSqr(r, g, b, i, j, k)
						// check that this point is not an interior point (one that was checked last time)
						// then check that it hasn't been used
						// then check that it's closed than the closest minimum
						if this_dist_sqr > previous_shell_sqr {
							if !colours[i][j][k] && this_dist_sqr < min_dist_sqr {
								min_dist_sqr = this_dist_sqr
								red = i
								green = j
								blue = k
								found = true
							}
						} else {
							// this_dist < outer radius - 1: this point was picked up in the previous shell
							// loop on the similar points across the axis relative to the point, then break
							// this is p hacky
							num_checked := k - maxint(0, b-int32(segment_level))
							for k = b + int32(segment_level) - num_checked; k < b+int32(segment_level) && k < MaxBlue; k++ {
								if !colours[i][j][k] && this_dist_sqr < min_dist_sqr {
									min_dist_sqr = this_dist_sqr
									red = i
									green = j
									blue = k
									found = true
								}
							}
							break
						}
					}
				}
			}
		}

	}

	colours[red][green][blue] = true
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

	FirstRed = int32(p_red)
	FirstGreen = int32(p_green)
	FirstBlue = int32(p_blue)
	TargetRadius = int32(blur)
	ChanSize = int32(ch_cap)
	if PicName == "" {
		PicName = fmt.Sprintf("art.r%dg%db%d.x%dy%d.blur%d.ch%d.cpu%d.png", FirstRed, FirstGreen, FirstBlue, StartX, StartY, TargetRadius, ChanSize, runtime.GOMAXPROCS(0))
	}

	colours := new(RGBCube)

	picture := new(PixelArray)

	// changing channel size affects behaviour of colour filling;
	// or rather, it makes CPU scheduling choices have a greater impact
	ch := make(chan image.Point, ChanSize)
	// seed point
	ch <- image.Pt(StartX, StartY)
	picture[StartX][StartY].Queued = true // pretend it got queued naturally
	_ = fillPixelArray(picture, colours, ch)

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
