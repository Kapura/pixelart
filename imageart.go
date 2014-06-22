package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const (
	// caps: red < MaxRed
	MaxRed   = 256
	MaxGreen = 256
	MaxBlue  = 256

	// filled alpha value: a = FullAlpha
	FullAlpha = 255

	MaxHeight = 4096
	MaxWidth  = 4096

	// r-g-b value of first seed point
	FirstRed   = 0
	FirstGreen = 0
	FirstBlue  = 0

	StartX = 0
	StartY = 0

	ChanSize = 8

	PicName = "art.png"
)

func sqr(x int32) int32 {
	return x * x
}

func distance(a, b, c, x, y, z int32) (d float64) {
	d = math.Sqrt(float64(sqr(a-x) + sqr(b-y) + sqr(c-z)))
	return
}

func mhtnDistance(a, b, c, x, y, z int32) (d float64) {
	d = math.Abs(float64(a-x)) + math.Abs(float64(b-y)) + math.Abs(float64(c-z))
	return
}

func maxint(a, b int32) int32 {
	if b > a {
		return b
	}
	return a
}

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

		if count%(MaxWidth*MaxHeight/8) == 0 {
			fmt.Printf("count: %d | Painting %d, %d, %d\n", count, red, green, blue)
		}
		pArray.Set(int32(point.X), int32(point.Y), red, green, blue)

		go repopulateChannel(ch, point, pArray)
	}
	return
}

func repopulateChannel(ch chan image.Point, point image.Point, pArray *PixelArray) {
	// repopulate channel wish FREE, NONQUEUED surrounding points

	for x_offset := -1; x_offset < 2; x_offset++ {
		if point.X+x_offset < MaxWidth && point.X+x_offset >= 0 {
			for y_offset := -1; y_offset < 2; y_offset++ {
				if point.Y+y_offset < MaxHeight && point.Y+y_offset >= 0 && !(x_offset == 0 && y_offset == 0) {
					pt := image.Pt(point.X+x_offset, point.Y+y_offset)
					if !pArray.FilledAt(int32(pt.X), int32(pt.Y)) && !pArray.QueuedAt(int32(pt.X), int32(pt.Y)) {
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
		outer_radius := int32(0)             // radius of the search sphere
		min_dist := float64(math.MaxFloat64) // initialised farther than largest possible radius
		found := false
		for !found && outer_radius < MaxRed {
			outer_radius++
			// expand in sphere about src pt until a colour is false (that is to say, unused)
			for i := maxint(0, r-outer_radius); i < r+outer_radius && i < MaxRed; i++ {
				// i traverses the end to end height of the sphere one level at a time i ~ r
				inner_radius := math.Sqrt(float64(sqr(outer_radius) + sqr(i-r)))
				// inner_radius = radius of (green-blue) circle at level i along sphere
				for j := maxint(0, g-int32(inner_radius)); j < g+int32(inner_radius) && j < MaxGreen; j++ {
					// j traverses the end to end width of the gb circle along the g axis one row at a time j ~ g
					segment_level := math.Sqrt(float64(sqr(int32(inner_radius)) + sqr(j-g)))
					for k := maxint(0, b-int32(segment_level)); k < b+int32(segment_level) && k < MaxBlue; k++ { // k ~ b
						this_dist := distance(r, g, b, i, j, k)
						// check that this point is not an interior point (one that was checked last time)
						// then check that it hasn't been used
						// then check that it's closed than the closest minimum
						if this_dist > float64(outer_radius-1) {
							if !colours[i][j][k] && this_dist < min_dist {
								min_dist = this_dist
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
								if !colours[i][j][k] && this_dist < min_dist {
									min_dist = this_dist
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

// cube representing png values in 3space
// becomes true when a colour is placed in the image
type RGBCube [MaxRed][MaxGreen][MaxBlue]bool

type Pixel struct {
	Colour struct {
		red   int32
		green int32
		blue  int32
	}
	Filled bool
	Queued bool
}

func (c *Pixel) NRGBA() *color.NRGBA {
	return &color.NRGBA{uint8(c.Colour.red), uint8(c.Colour.green), uint8(c.Colour.blue), FullAlpha}
}

// PixelArray is an array of the Pixel objects, and should be used to
// interface with the underlying Pixel structure
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
	c[x][y].Colour.red = red
	c[x][y].Colour.green = green
	c[x][y].Colour.blue = blue
	c[x][y].Filled = true
}

func (c *PixelArray) ColourAt(x, y int32) (red, green, blue int32) {
	red = c[x][y].Colour.red
	green = c[x][y].Colour.green
	blue = c[x][y].Colour.blue
	return
}

func (c *PixelArray) FilledAt(x, y int32) bool {
	return c[x][y].Filled
}

func (c *PixelArray) QueuedAt(x, y int32) bool {
	return c[x][y].Queued
}

func (c *PixelArray) TargetColourAt(x, y int32) (red, green, blue int32) {

	var r_sum, g_sum, b_sum int32 // temps to hold the colours of surrounding pixels
	var filled_neighbours int32
	var x_offset, y_offset int32

	for x_offset = -1; x_offset < 2; x_offset++ {
		if x+x_offset < MaxWidth && x+x_offset >= 0 {
			for y_offset = -1; y_offset < 2; y_offset++ {

				if x_offset == 0 && y_offset == 0 {
					//skip calling pixel
					continue
				}

				if y+y_offset < MaxHeight && y+y_offset >= 0 {
					if c.FilledAt(x+x_offset, y+y_offset) {
						r, g, b := c.ColourAt(x+x_offset, y+y_offset)
						r_sum += r
						g_sum += g
						b_sum += b
						filled_neighbours++
					}
				}
			}
		}
	}

	if filled_neighbours > 0 {
		red = r_sum / filled_neighbours
		green = g_sum / filled_neighbours
		blue = b_sum / filled_neighbours
	}
	return
}

// main function does all the stuff in the right order
func main() {
	colours := new(RGBCube)

	picture := new(PixelArray)

	// changing channel size affects behaviour of colour filling;
	// or rather, it makes CPU scheduling choices have a greater impact
	ch := make(chan image.Point, ChanSize)
	// seed point
	ch <- image.Pt(StartX, StartY)
	picture[StartX][StartY].Queued = true // pretend it got popped naturally
	_ = fillPixelArray(picture, colours, ch)

	draw(picture.ImageNRGBA())
}
