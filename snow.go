package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strconv"

	"github.com/anthonynsimon/bild/adjust"
	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"

	"github.com/aquilax/go-perlin"
)

// matrix size, this decides the size of the image
const size int = 800

// note:
// I use a 2 dimentional matrix and treats it as a hexagonial grid.
//
// X X X X X X X X X X X X
// X X X X . . . . . . . X
// X X X . . . . . . . . X
// X X . . . N N . . . . X
// X . . . N O N . . . . X
// X . . . N N . . . . X X
// X . . . . . . . . X X X
// X . . . . . . . X X X X
// X X X X X X X X X X X X
//
// where X is out of bound, O is is a frozen hexagon and N it's neighbours.
// When the matrix values have become pixel values the image gets sheared to make it look normal.

type Matrix [size][size]float64
type Mask [size][size]uint8

// states the mask can have
const (
	receptive uint8 = iota
	non_receptive
	out_of_bound
)

func main() {
	// A, B, Y, PP, PM, L parameters
	args := os.Args[1:]
	A, _ := strconv.ParseFloat(args[0], 64)
	B, _ := strconv.ParseFloat(args[1], 64)
	Y, _ := strconv.ParseFloat(args[2], 64)
	PP, _ := strconv.ParseFloat(args[3], 64)
	PM, _ := strconv.ParseFloat(args[4], 64)
	L, _ := strconv.ParseInt(args[5], 10, 64)

	fmt.Printf("settings:\t A=%.4f B=%.4f Y=%.4f PP=%.4f PM=%.4f I=%d size=%d\n", A, B, Y, PP, PM, L, size)

	// create matrices
	var coldness_matrix Matrix
	var mask_matrix Mask
	init_matrices(B, PP, PM, &coldness_matrix, &mask_matrix)

	// run simulation loop
	for iteration := int64(0); iteration <= L; iteration++ {
		step(A, B, Y, &coldness_matrix, &mask_matrix)
		fmt.Printf("\rsimulation:\t %d / %d", iteration, L)
	}

	// save as png
	filename := fmt.Sprintf("snowflakes/%.4f-%.4f-%.4f-%.4f-%.4f-%d-%d.png", A, B, Y, PP, PM, L, size)
	save(filename, &coldness_matrix)
	fmt.Println("\nsaved result:\t", filename)
}

func init_matrices(B, PP, PM float64, coldness_matrix *Matrix, mask_matrix *Mask) {
	// perlin noise generator
	perlin := perlin.NewPerlin(2, 2, 1, 1)

	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			// set coldness initial background level, B, PP, PM parameters are used here
			perlin_value := perlin.Noise2D(float64(i)*PP, float64(j)*PP) * PM
			coldness_matrix[i][j] = perlin_value + B

			// set a border for the matrix where no calculation is done
			x := i - size/2
			z := j - size/2
			y := -x - z

			if math.Max(math.Max(math.Abs(float64(x)), math.Abs(float64(y))), math.Abs(float64(z))) > float64(size/2-2) {
				mask_matrix[i][j] = out_of_bound
			} else {
				// all hexagons are set to non receptive at the beginning because there are no frozen hexagons
				mask_matrix[i][j] = non_receptive
			}
		}
	}

	// freeze the middle hexagon
	coldness_matrix[size/2][size/2] = 1.0
}

func step(A, B, Y float64, coldness_matrix *Matrix, mask_matrix *Mask) {
	// look for frozen hexagons and set receptive values on the mask
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			if (*coldness_matrix)[i][j] >= 1.0 {
				(*mask_matrix)[i-1][j] = receptive
				(*mask_matrix)[i-1][j+1] = receptive
				(*mask_matrix)[i][j-1] = receptive
				(*mask_matrix)[i][j] = receptive
				(*mask_matrix)[i][j+1] = receptive
				(*mask_matrix)[i+1][j-1] = receptive
				(*mask_matrix)[i+1][j] = receptive
			}
		}
	}

	// create next itteration of the coldness matrix
	var temp_coldness_matrix Matrix

	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			switch {
			case (*mask_matrix)[i][j] == non_receptive:
				// simulate water floating out to it's neighbour hexagons
				v0 := (*coldness_matrix)[i][j]
				v1 := A * v0 / 12.0

				temp_coldness_matrix[i-1][j] += v1
				temp_coldness_matrix[i-1][j+1] += v1
				temp_coldness_matrix[i][j-1] += v1
				temp_coldness_matrix[i][j] += v0 / 2.0
				temp_coldness_matrix[i][j+1] += v1
				temp_coldness_matrix[i+1][j-1] += v1
				temp_coldness_matrix[i+1][j] += v1

			case (*mask_matrix)[i][j] == receptive:
				// add constant to hexagons next to already frozen hexagon
				temp_coldness_matrix[i][j] += (*coldness_matrix)[i][j] + Y

			default:
				// ignore out of bound
			}
		}
	}

	// overwrite the old coldness matrix with the new coldness matrix
	*coldness_matrix = temp_coldness_matrix
}

func save(filename string, matrix *Matrix) {
	// create empty canvas
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// draw coldness matrixs values to pixels
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			c := math.Min((matrix[x][y] * 255), 255)
			img.Set(x, y, color.Gray{uint8(c)})
		}
	}

	// shear the image horizontally
	img = transform.ShearH(img, -30)

	// crop it in the middle (magic to find the middle after the shear)
	c := float64(size) / math.Cos(math.Pi/6.0)
	a := math.Sqrt(math.Pow(c, 2) - math.Pow(float64(size), 2))
	img = transform.Crop(img, image.Rect(int(a/2), 0, int(a/2)+size, size))

	// fill out the emptyness from the shear
	img = adjust.Apply(img, func(r color.RGBA) color.RGBA {
		r.A = 255
		return r
	})

	imgio.Save(filename, img, imgio.PNGEncoder())
}
