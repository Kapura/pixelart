package main

import (
	"fmt"
	"image"
	"math/rand"
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

///// the rest of the code and whatnot

func fillPixelArray(pArray *PixelArray, cspace Colourspace, seedCh chan SeedPixel, ch chan image.Point, args GenerateArgs) (count int32) {
	// for printing intermediate images
	origPicName := args.name
	var ir_tag int32 = 1
	var pic_fraction int32 = int32(args.width*args.height) / args.update_freq
	var tmp_colour Colour24

	var seeds int32 = 0
	//put in the seed pixels
	rand.Seed(time.Now().UnixNano())
	for {
		sp, more := <-seedCh
		if more {
			// seed pixel from channel

			if !cspace.ColourUsed(sp) || args.reseed_dupes {
				seeds++
				tmp_colour = cspace.PopColour(sp)

				pArray.Set(int32(sp.Pt.X), int32(sp.Pt.Y), tmp_colour)

				go func() {
					for x_offset := -1; x_offset < 2; x_offset++ {
						if sp.Pt.X+x_offset < args.width && sp.Pt.X+x_offset >= 0 {
							for y_offset := -1; y_offset < 2; y_offset++ {
								if sp.Pt.Y+y_offset < args.height && sp.Pt.Y+y_offset >= 0 && !(x_offset == 0 && y_offset == 0) {
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

	for count = seeds; count < int32(args.width*args.height); count++ {
		point := <-ch

		//in the case of a point being re-queued after being filled
		//TODO: assert !FilledAt
		if pArray.FilledAt(int32(point.X), int32(point.Y)) {
			count--
			continue
		}

		tmp_colour = pArray.TargetColourAt(int32(point.X), int32(point.Y), args.blur, args.width, args.height)

		tmp_colour = cspace.PopColour(tmp_colour)

		// it's nice to know the algorithm is running
		if count > ir_tag*pic_fraction && ir_tag < args.update_freq {
			fmt.Printf("[%s] %2.1f%% of pixels filled\n", timestamp(), float64(count * 100)/float64(args.width * args.height))
			if args.draw_ir {
				name := fmt.Sprintf("%s.%3d.png", args.tag, ir_tag)
				go draw(pArray.ImageNRGBA(args.width, args.height, args.flip_draw), name)
			}
			ir_tag++

			if args.update != nil {
				go args.update(pArray.ImageNRGBA(args.width, args.height, args.flip_draw))
			}

		}

		if count == MaxWidth*MaxHeight*15/16 {
			fmt.Println("Endgame optimisation... (this last one takes the longest :( )")
			cspace.PrepOpt()
		}

		pArray.Set(int32(point.X), int32(point.Y), tmp_colour)

		go func() {
			for x_offset := -1; x_offset < 2; x_offset++ {
				if point.X+x_offset < args.width && point.X+x_offset >= 0 {
					for y_offset := -1; y_offset < 2; y_offset++ {
						if point.Y+y_offset < args.height && point.Y+y_offset >= 0 && !(x_offset == 0 && y_offset == 0) {
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

	args.name = origPicName

	if args.update != nil {
		go args.update(pArray.ImageNRGBA(args.width, args.height, args.flip_draw))
	}

	return
}

func processSeedImage(seedCh chan SeedPixel, args GenerateArgs) {
	bounds := args.seed_image.Bounds()
	// spiral seeding

	checkAndSeed := func(x, y int) { // if pixel != chroma, add to seed queue
		var pixel SeedPixel
		r, g, b, _ := args.seed_image.At(x, y).RGBA()
		if (r/256<<16)|(g/256<<8)|b/256 != uint32(args.chroma_colour) {
			// randomly cull a set percentage to get more balanced images
			if args.seed_rejection_rate > 0 {
				if rand.Float64() > args.seed_rejection_rate {
					if args.flip_draw {
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
	checkAndSeed(args.start_x, args.start_y)

	var x, y int
	var layer int = 0
	var xMinCap, xMaxCap, yMinCap, yMaxCap bool = false, false, false, false
	var start, end int

	// while not at all four boundaries, scan the image
	for !xMinCap || !xMaxCap || !yMinCap || !yMaxCap {
		layer++

		// topright to topleft
		y = args.start_y - layer
		if y < bounds.Min.Y {
			yMinCap = true
		} else {
			start = minint(args.start_x+layer, bounds.Max.X-1)
			end = maxint(args.start_x-layer, bounds.Min.X)
			scanLine(start, end, y, y)
		}

		//topleft to botleft
		x = args.start_x - layer
		if x < bounds.Min.X {
			xMinCap = true
		} else {
			start = maxint(args.start_y-layer, bounds.Min.Y)
			end = minint(args.start_y+layer, bounds.Max.Y-1)
			scanLine(x, x, start, end)
		}

		// botleft to botright
		y = args.start_y + layer
		if y >= bounds.Max.Y {
			yMaxCap = true
		} else {
			start = maxint(args.start_x-layer, bounds.Min.X)
			end = minint(args.start_x+layer, bounds.Max.X-1)
			scanLine(start, end, y, y)
		}

		//botright to topright
		x = args.start_x + layer
		if x >= bounds.Max.X {
			xMaxCap = true
		} else {
			start = minint(args.start_y+layer, bounds.Max.Y-1)
			end = maxint(args.start_y-layer, 0)
			scanLine(x, x, start, end)
		}
	}
	close(seedCh)
}

func composeImageName(args GenerateArgs) (name string) {
	name = fmt.Sprintf("%s.%s", args.tag, ToString(args.colour_basis))

	if args.seed_image == nil {
		name += fmt.Sprintf(".r%dg%db%d", args.start_red, args.start_green, args.start_blue)
	} else {
		name += fmt.Sprintf(".rr%1.3f", args.seed_rejection_rate)
	}

	name += fmt.Sprintf(".x%dy%d.blur%d.ch%d.cpu%d", args.start_x, args.start_y, args.blur, args.chan_size, runtime.GOMAXPROCS(0))

	if args.flip_draw {
		name += ".flip"
	}

	if args.echospace > 0 {
		name += fmt.Sprintf(".es%1.5f", args.echospace)
	}

	name += ".png"
	return
}

type GenerateArgs struct {
	cpus int
	chan_size int32
	blur int32
	colour_basis ColourBasis
	echospace float64
	flip_draw bool
	draw_ir bool

	name string
	tag string

	seed_image image.Image
	seed_rejection_rate float64
	reseed_dupes bool
	chroma_colour int

	start_red int
	start_green int
	start_blue int
	start_x int
	start_y int

	height int
	width int

	update func(pic image.Image)
	update_freq int32
}

func Generate(args GenerateArgs) {
	// Also affects CPU scheduling I suppose :)
	if args.cpus != 0 {
		if (args.cpus < 0) || (args.cpus > runtime.NumCPU()) {
			args.cpus = runtime.NumCPU()
		}
		runtime.GOMAXPROCS(args.cpus)
		args.cpus = runtime.GOMAXPROCS(0)
	}

	var seedCh chan SeedPixel

	var colours Colourspace = GetColourspace(args.colour_basis)
	colours.SetEchospace(args.echospace)

	picture := new(PixelArray)

	if args.seed_image != nil {
		bounds := args.seed_image.Bounds()
		chanSize := (bounds.Max.X * bounds.Max.Y)
		if args.seed_rejection_rate > 0 {
			chanSize = int(float64(chanSize) * (1 - (args.seed_rejection_rate * args.seed_rejection_rate)))
		}
		seedCh = make(chan SeedPixel, chanSize)
		go processSeedImage(seedCh, args)

	} else {
		// seeding based on params rather than seed image
		seedCh = make(chan SeedPixel, 1)
		seedCh <- NewSeedPixel(
			uint8(args.start_red),
			uint8(args.start_green),
			uint8(args.start_blue),
			args.start_x,
			args.start_y)

		close(seedCh)
	}

	if args.name == "" {
		args.name = composeImageName(args)
	}

	// changing channel size affects behaviour of colour filling;
	// or rather, it makes CPU scheduling choices have a greater impact
	ch := make(chan image.Point, args.chan_size)
	_ = fillPixelArray(picture, colours, seedCh, ch, args)

	draw(picture.ImageNRGBA(args.width, args.height, args.flip_draw), args.name)
}
