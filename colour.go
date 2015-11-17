package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

type Colour interface {
	Red() int32
	Green() int32
	Blue() int32
}

type Colour24 struct {
	red   uint8
	green uint8
	blue  uint8
}

func (c Colour24) Red() int32 {
	return int32(c.red)
}

func (c Colour24) Green() int32 {
	return int32(c.green)
}

func (c Colour24) Blue() int32 {
	return int32(c.blue)
}

// location-pixel meta-struct for seed image
type SeedPixel struct {
	Colour Colour24
	Pt     image.Point
}

func NewSeedPixel(r, g, b uint8, x, y int) (spixel SeedPixel) {
	spixel.Colour = Colour24{r, g, b}
	spixel.Pt = image.Point{x, y}
	return
}

func (c SeedPixel) Red() int32 {
	return c.Colour.Red()
}

func (c SeedPixel) Green() int32 {
	return c.Colour.Green()
}

func (c SeedPixel) Blue() int32 {
	return c.Colour.Blue()
}

// Pixel struct for filling in image
type Pixel struct {
	Colour Colour24
	Filled bool
	Queued bool
}

func (c Pixel) Red() int32 {
	return c.Colour.Red()
}

func (c Pixel) Green() int32 {
	return c.Colour.Green()
}

func (c Pixel) Blue() int32 {
	return c.Colour.Blue()
}

func (c *Pixel) NRGBA(args GenerateArgs) *color.NRGBA {
	if args.flip_draw {
		return &color.NRGBA{255 - c.Colour.red, 255 - c.Colour.green, 255 - c.Colour.blue, FullAlpha}
	} else {
		return &color.NRGBA{c.Colour.red, c.Colour.green, c.Colour.blue, FullAlpha}
	}
}

// representation of the image as a 2D array
type PixelArray [MaxWidth][MaxHeight]Pixel

func (p *PixelArray) ImageNRGBA(args GenerateArgs) *image.NRGBA {
	pic := image.NewNRGBA(image.Rect(0, 0, args.width, args.height))
	for x := 0; x < args.width; x++ {
		for y := 0; y < args.height; y++ {
			pic.Set(x, y, p[x][y].NRGBA(args))
		}
	}
	return pic
}

func (p *PixelArray) Set(x, y int32, c Colour) {
	p[x][y].Colour.red = uint8(c.Red())
	p[x][y].Colour.green = uint8(c.Green())
	p[x][y].Colour.blue = uint8(c.Blue())
	p[x][y].Queued = true
	p[x][y].Filled = true
}

func (p *PixelArray) ColourAt(x, y int32) Colour24 {
	return p[x][y].Colour
}

func (p *PixelArray) FilledAt(x, y int32) bool {
	return p[x][y].Filled
}

func (p *PixelArray) QueuedAt(x, y int32) bool {
	return p[x][y].Queued
}

// TargetColourAt examines the neighbouring pixels to the point (x, y) and
// returns an unweighted average of the red, green, and blue channels of
// the filled nearby pixels.
func (c *PixelArray) TargetColourAt(x, y int32, args GenerateArgs) Colour24 {

	var r_sum, g_sum, b_sum float64 // temps to hold the colours of surrounding pixels
	var filled_neighbours float64
	var x_offset, y_offset int32

	for x_offset = -args.blur; x_offset <= args.blur; x_offset++ {
		if x+x_offset < int32(args.width) && x+x_offset >= 0 {
			for y_offset = -args.blur; y_offset <= args.blur; y_offset++ {

				if x_offset == 0 && y_offset == 0 {
					//skip calling pixel
					continue
				}

				if y+y_offset < int32(args.height) && y+y_offset >= 0 {
					if c.FilledAt(x+x_offset, y+y_offset) {
						k := c.ColourAt(x+x_offset, y+y_offset)
						r_sum += float64(k.red)
						g_sum += float64(k.green)
						b_sum += float64(k.blue)
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
		red := uint8(math.Floor(r_sum + 0.5))
		green := uint8(math.Floor(g_sum + 0.5))
		blue := uint8(math.Floor(b_sum + 0.5))
		return Colour24{red, green, blue}
	}
	return Colour24{0, 0, 0}
}

// draw function for the final image
func draw(pic *image.NRGBA, args GenerateArgs) {
	fmt.Println("Drawing", args.name)
	file, err := os.Create(args.name)

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
