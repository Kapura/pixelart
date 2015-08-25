package main

import (
	"fmt"
	"math"
)

type ColourBasis uint8

//colourspaces
const (
	RGB ColourBasis = iota
	RBG
	GBR
	GRB
	BGR
	BRG
)

type Colourspace interface {
	ColourUsed(r, g, b int32) bool
	GetMaxColourCount() int32
	GetColourCount() int32
	PopColour(r, g, b int32) (red, green, blue int32)
	PrepOpt()
	SetEchospace(value float64)
}

type multiColourSpace struct {
	colourBasis               ColourBasis
	optimised                 bool
	echoStartPoint            int32
	echoQueue                 *ColourQueue
	RGBCube                   [256][256][256]bool
	count                     int32
	xCounts, yCounts, zCounts [256]int32
}

func GetColourspace(basis ColourBasis) Colourspace {
	space := new(multiColourSpace)
	space.colourBasis = basis
	space.optimised = false
	return space
}

func (space *multiColourSpace) ColourUsed(r, g, b int32) bool {
	switch space.colourBasis {
	case RGB:
		return space.RGBCube[r][g][b]
	case RBG:
		return space.RGBCube[r][b][g]
	case GBR:
		return space.RGBCube[g][b][r]
	case GRB:
		return space.RGBCube[g][r][b]
	case BGR:
		return space.RGBCube[b][g][r]
	case BRG:
		return space.RGBCube[b][r][g]
	}
	return false
}

func (space *multiColourSpace) GetMaxColourCount() int32 {
	return int32(256 * 256 * 256)
}

func (space *multiColourSpace) GetColourCount() int32 {
	return space.count
}

func (space *multiColourSpace) SetEchospace(value float64) {
	if value == 0 {
		space.echoStartPoint = 0
	} else {
		space.echoStartPoint = int32(value * float64(space.GetMaxColourCount()))
		space.echoQueue = NewColourQueue(int(space.echoStartPoint + 1))
	}
}

func (space *multiColourSpace) PrepOpt() {
	if space.echoStartPoint <= 0 {
		for x := 0; x < 256; x++ {
			for y := 0; y < 256; y++ {
				for z := 0; z < 256; z++ {
					if !space.RGBCube[x][y][z] {
						space.xCounts[x]++
						space.yCounts[y]++
						space.zCounts[z]++
					}
				}
			}
		}
		space.optimised = true
	}
}

func (space *multiColourSpace) PopColour(r, g, b int32) (red, green, blue int32) {

	if space.optimised && space.echoStartPoint <= 0 {

		switch space.colourBasis {
		case RGB:
			red, green, blue = space.basePopColourOpt(r, g, b)
		case RBG:
			red, blue, green = space.basePopColourOpt(r, b, g)
		case GBR:
			green, blue, red = space.basePopColourOpt(g, b, r)
		case GRB:
			green, red, blue = space.basePopColourOpt(g, r, b)
		case BGR:
			blue, green, red = space.basePopColourOpt(b, g, r)
		case BRG:
			blue, red, green = space.basePopColourOpt(b, r, g)
		}

	} else {

		switch space.colourBasis {
		case RGB:
			red, green, blue = space.basePopColour(r, g, b)
		case RBG:
			red, blue, green = space.basePopColour(r, b, g)
		case GBR:
			green, blue, red = space.basePopColour(g, b, r)
		case GRB:
			green, red, blue = space.basePopColour(g, r, b)
		case BGR:
			blue, green, red = space.basePopColour(b, g, r)
		case BRG:
			blue, red, green = space.basePopColour(b, r, g)
		}
	}

	return
}

func (space *multiColourSpace) basePopColour(r, g, b int32) (red, green, blue int32) {

	// check the colour cube to see if the colour has been painted yet
	if !space.RGBCube[r][g][b] {
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
						this_dist_sqr := float64(distSqr(r, g, b, i, j, k))
						// check that this point is not an interior point (one that was checked last time)
						// then check that it hasn't been used
						// then check that it's closed than the closest minimum
						if this_dist_sqr > previous_shell_sqr {
							if !space.RGBCube[i][j][k] && this_dist_sqr < min_dist_sqr {
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
								if !space.RGBCube[i][j][k] && this_dist_sqr < min_dist_sqr {
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

	space.RGBCube[red][green][blue] = true
	space.count++

	if space.echoStartPoint > 0 {
		space.echoQueue.Push(Colour24{uint8(red), uint8(green), uint8(blue)})
		if space.count >= space.echoStartPoint {
			echo := space.echoQueue.Pop()
			space.RGBCube[echo.red][echo.green][echo.blue] = false
		}

	}

	return
}

func (space *multiColourSpace) basePopColourOpt(r, g, b int32) (red, green, blue int32) {

	// check the colour cube to see if the colour has been painted yet
	if !space.RGBCube[r][g][b] {
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
				if space.xCounts[i] <= 0 {
					continue
				}
				// i traverses the end to end height of the sphere one level at a time i ~ r
				inner_radius := math.Sqrt(float64(sqr(outer_radius) + sqr(i-r)))
				// inner_radius = radius of (green-blue) circle at level i along sphere
				for j := maxint(0, g-int32(inner_radius)); j < g+int32(inner_radius) && j < MaxGreen; j++ {
					if space.yCounts[j] <= 0 {
						continue
					}
					// j traverses the end to end width of the gb circle along the g axis one row at a time j ~ g
					segment_level := math.Sqrt(float64(sqr(int32(inner_radius)) + sqr(j-g)))
					for k := maxint(0, b-int32(segment_level)); k < b+int32(segment_level) && k < MaxBlue; k++ { // k ~ b
						if space.zCounts[k] <= 0 {
							continue
						}
						this_dist_sqr := float64(distSqr(r, g, b, i, j, k))
						// check that this point is not an interior point (one that was checked last time)
						// then check that it hasn't been used
						// then check that it's closed than the closest minimum
						if this_dist_sqr > previous_shell_sqr {
							if !space.RGBCube[i][j][k] && this_dist_sqr < min_dist_sqr {
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
								if !space.RGBCube[i][j][k] && this_dist_sqr < min_dist_sqr {
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

	space.xCounts[red]--
	space.yCounts[green]--
	space.zCounts[blue]--
	space.RGBCube[red][green][blue] = true
	space.count++
	return
}

// Queue shit from https://gist.github.com/moraes/2141121

// NewQueue returns a new queue with the given initial size.
func NewColourQueue(size int) *ColourQueue {
	return &ColourQueue{
		nodes: make([]Colour24, size),
		size:  size,
	}
}

// Queue is a basic FIFO queue based on a circular list that resizes as needed.
type ColourQueue struct {
	nodes []Colour24
	size  int
	head  int
	tail  int
	count int
}

// Push adds a node to the queue.
func (q *ColourQueue) Push(n Colour24) {
	if q.head == q.tail && q.count > 0 {
		nodes := make([]Colour24, len(q.nodes)+q.size)
		copy(nodes, q.nodes[q.head:])
		copy(nodes[len(q.nodes)-q.head:], q.nodes[:q.head])
		q.head = 0
		q.tail = len(q.nodes)
		q.nodes = nodes
	}
	q.nodes[q.tail] = n
	q.tail = (q.tail + 1) % len(q.nodes)
	q.count++
}

// Pop removes and returns a node from the queue in first to last order.
func (q *ColourQueue) Pop() Colour24 {
	if q.count == 0 {
		panic(fmt.Errorf("%s", "Error: trying to pop empty ColourQueue"))
	}
	node := q.nodes[q.head]
	q.head = (q.head + 1) % len(q.nodes)
	q.count--
	return node
}
