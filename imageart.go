package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func distance(a, b, c, x, y, z int32) (d float64) {
	d = math.Sqrt(math.Pow(float64(a-x), 2) + math.Pow(float64(b-y), 2) + math.Pow(float64(c-z), 2))
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
	file, err := os.Create("newpic.png")

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
	fmt.Println("Done.")
}

func popAndDraw(pic *image.NRGBA, colours *RGBCube, c chan image.Point, queuedList *QueuedArray) {
	for count := 0; count < 256*256*256; count++ {
		point := <-c
		// check that it hasn't already been filled
		_, _, _, a := pic.At(point.X, point.Y).RGBA()
		if a > 0 {
			continue
		}
		// got one
		// guaranteed to have SOME neighbours unless it's the first point
		var red, green, blue int32
		if count == 0 {
			red = 0
			green = 0
			blue = 0
		} else {
			red, green, blue = getTargetColour(point.X, point.Y, pic)
		}
		fmt.Printf("target: %d %d %d\n", red, green, blue)
		red, green, blue = getColourFromTarget(red, green, blue, colours)
		fmt.Printf("using: %d %d %d\n", red, green, blue)

		pic.Set(point.X, point.Y, color.NRGBA{uint8(red), uint8(green), uint8(blue), 255})

		go repopulateChannel(c, point, pic, queuedList)
	}
	draw(pic)
}

func repopulateChannel(c chan image.Point, point image.Point, pic *image.NRGBA, queuedList *QueuedArray) {
	n := 0
	pt_array := make([]image.Point, 8)
	// fill pt_array with valid neighbours
	for i := -1; i < 2; i++ {
		if point.X+i < 4096 && point.X+i >= 0 {
			for j := -1; j < 2; j++ {
				if point.Y+j < 4096 && point.Y+j >= 0 && !(i == 0 && j == 0) {
					pt_array[n] = image.Pt(point.X+i, point.Y+j)
					n++
				}
			}
		}
	}

	for i := 0; i < n; i++ {
		pt := pt_array[i]
		current := pic.At(pt.X, pt.Y)
		_, _, _, a := current.RGBA()
		if a == 0 && !queuedList[pt.X][pt.Y] {
			fmt.Printf("Adding %d, %d\n", pt.X, pt.Y)
			queuedList[pt.X][pt.Y] = true
			c <- pt
		}
	}
}

func getColourFromTarget(r, g, b int32, colours *RGBCube) (red, green, blue int32) {
	if !colours[r][g][b] {
		red = r
		green = g
		blue = b
	} else {
		// true = colour already used, near to find closest unused colour
		outer_radius := int32(0) // radius of the search sphere
		min_dist := float64(7095)
		found := false
		for !found && outer_radius < 256 {
			outer_radius++
			// expand in sphere about src pt until a colour is false (that is to say, unused)
			for i := maxint(0, r-outer_radius); i < r+outer_radius && i < 256; i++ {
				// i traverses the end to end height of the sphere one level at a time i ~ r
				inner_radius := math.Sqrt(math.Pow(float64(outer_radius), 2) + math.Pow(float64(i-r), 2))
				// inner_radius = radius of (gb) circle at level i along sphere
				for j := maxint(0, g-int32(inner_radius)); j < g+int32(inner_radius) && j < 256; j++ {
					// j traverses the end to end width of the gb circle along the g axis one row at a time j ~ g
					secant_level := math.Sqrt(math.Pow(inner_radius, 2) + math.Pow(float64(j-g), 2))
					for k := maxint(0, b-int32(secant_level)); k < b+int32(secant_level) && k < 256; k++ {
						this_dist := distance(r, g, b, i, j, k)
						if this_dist > float64(outer_radius-1) && !colours[i][j][k] && this_dist < min_dist {
							min_dist = this_dist
							red = i
							green = j
							blue = k
							found = true
						}
					}
				}
			}
		}

	}

	colours[red][green][blue] = true
	return
}

func getTargetColour(x, y int, pic *image.NRGBA) (red, green, blue int32) {

	n := 0
	pt_array := make([]image.Point, 8)
	// fill pt_array with valid neighbours
	for i := -1; i < 2; i++ {
		if x+i < 4096 && x+i >= 0 {
			for j := -1; j < 2; j++ {
				if y+j < 4096 && y+j >= 0 && !(i == 0 && j == 0) {
					pt_array[n] = image.Pt(x+i, y+j)
					n++
				}
			}
		}
	}

	k := int32(0)
	red = 0
	green = 0
	blue = 0
	for i := 0; i < n; i++ {
		pt := pt_array[i]
		current := pic.At(pt.X, pt.Y)
		r, g, b, a := current.RGBA()
		r /= 256
		g /= 256
		b /= 256
		fmt.Printf("r=%d g=%d b=%d a=%d\n", r, g, b, a)
		if a > 0 {
			k++
			red += int32(r)
			green += int32(g)
			blue += int32(b)
		}
	}

	if k != 0 {
		red = red / k
		green = green / k
		blue = blue / k
	}
	return
}

type RGBCube [256][256][256]bool

type QueuedArray [4096][4096]bool

func main() {
	fmt.Println("starting...")
	height := 4096
	width := 4096

	pic := image.NewNRGBA(image.Rect(0, 0, width, height))

	colours := new(RGBCube)

	queuedList := new(QueuedArray)

	c := make(chan image.Point, height*width+1)
	c <- image.Pt(2048, 2048)
	queuedList[2048][2048] = true
	popAndDraw(pic, colours, c, queuedList)
}
