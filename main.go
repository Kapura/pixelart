package main

import (
	"flag"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"time"
)

func parseFlags() (args GenerateArgs){

	var(
	seedImagePath     string
	seedRejectionRate float64
	seedChroma        int
	seedDupes         bool

	echospacing float64

	seedColour int
	p_red      int
	p_green    int
	p_blue     int

	colourAxes  string

	x int
	y int

	width  int
	height int

	blur         int
	ch_cap       int
	cpu_cap      int

	draw_intermediate bool
	flip_draw         bool

	tag  string
	name string
	)

	flag.IntVar(&p_red, "seed-red", 0, "red value of the initial point")
	flag.IntVar(&p_green, "seed-green", 0, "green value of the initial point")
	flag.IntVar(&p_blue, "seed-blue", 0, "blue value of the initial point")

	flag.StringVar(&colourAxes, "colour-basis", "rgb", "colour axes to use: one of [rgb, rbg, gbr, grb, bgr, brg]")

	flag.IntVar(&width, "width", 4096, "Output image width (if not using seed image)")
	flag.IntVar(&height, "height", 4096, "Output image height (if not using seed image")

	flag.IntVar(&seedColour, "seed", 0x0, "seed colour (e.g. 0xFFFFFF)")
	flag.IntVar(&x, "seed-x", 0, "x position of the initial point")
	flag.IntVar(&y, "seed-y", 0, "y position of the initial point")
	flag.StringVar(&seedImagePath, "seed-image", "", "Pre-seeded image to fill. Empty pixels are 0x000000")
	flag.Float64Var(&seedRejectionRate, "seed-rr", 0, "Random rejection rate of seeded pixels between 0 and 1")
	flag.IntVar(&seedChroma, "seed-chroma-key", 0xFF00FF, "Colour to treat as empty in seeded image")
	flag.BoolVar(&seedDupes, "seed-dupes", false, "Search for repeated colours in input image. Takes a bit.")

	flag.IntVar(&blur, "blur", 1, "higher values increase time required to complete image.")

	flag.IntVar(&ch_cap, "chan", 8, "very high values produce geometric patterns originating about the initial point.")

	flag.StringVar(&name, "name", "", "name to use for final image file")
	flag.StringVar(&tag, "tag", "art", "tags for intermediate representation and final file (if no PicName specified)")

	flag.BoolVar(&draw_intermediate, "ir", false, "draw intermediate representations of the image")
	flag.BoolVar(&flip_draw, "flip-draw", false, "flip ALL colours at the bit level after running")

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
			args.seed_image, err = png.Decode(file)
		} else if extension == ".jpeg" || extension == ".jpg" {
			args.seed_image, err = jpeg.Decode(file)
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

		width = args.seed_image.Bounds().Max.X
		height = args.seed_image.Bounds().Max.Y
	}

	switch colourAxes {
	case "rgb":
		args.colour_basis = RGB
	case "rbg":
		args.colour_basis = RBG
	case "gbr":
		args.colour_basis = GBR
	case "grb":
		args.colour_basis = GRB
	case "bgr":
		args.colour_basis = BGR
	case "brg":
		args.colour_basis = BRG
	}

	args.chan_size = int32(ch_cap)
	args.cpus = cpu_cap
	args.blur = int32(blur)
	args.echospace = echospacing
	args.flip_draw = flip_draw
	args.draw_ir = draw_intermediate

	args.name = name
	args.tag = tag

	args.seed_rejection_rate = seedRejectionRate
	args.reseed_dupes = seedDupes
	args.chroma_colour = seedChroma

	args.start_red = p_red
	args.start_green = p_green
	args.start_blue = p_blue
	args.start_x = x
	args.start_y = y

	args.height = height
	args.width = width

	return

}

func timestamp() string {
	var hour, min, sec = time.Now().Clock()
	return fmt.Sprintf("%02d:%02d:%02d", hour, min, sec)
}

func main() {

	var start_hour, start_min, start_sec = time.Now().Clock()
	fmt.Printf("Start time: %d:%d:%d\n", start_hour, start_min, start_sec)

	args := parseFlags()

	Generate(args)

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
