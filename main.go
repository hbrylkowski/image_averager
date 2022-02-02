package main

import (
	"bufio"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"os"
	"path/filepath"
)

const INGEST_BUFFER = 256
const SUM_WORKERS = 2

type averageImage struct {
	pixels       [][][]uint32
	imagesSummed int
}

func (m *averageImage) toImage() image.Image {
	cimg := image.NewRGBA(image.Rect(0, 0, len(m.pixels[0]), len(m.pixels[0][0])))

	width := len(m.pixels[0])
	height := len(m.pixels[0][0])

	for w := 0; w < width; w++ {
		for h := 0; h < height; h++ {
			r := m.pixels[0][w][h] / uint32(m.imagesSummed)
			g := m.pixels[1][w][h] / uint32(m.imagesSummed)
			b := m.pixels[2][w][h] / uint32(m.imagesSummed)
			cimg.Set(w, h, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
		}
	}
	return cimg
}

func newAveragedImage(width int, height int) averageImage {
	averaged := averageImage{}
	for i := 0; i < 3; i++ {
		var row [][]uint32
		for w := 0; w < width; w++ {
			var column []uint32
			for h := 0; h < height; h++ {
				column = append(column, 0)
			}
			row = append(row, column)
		}
		averaged.pixels = append(averaged.pixels, row)
	}
	return averaged
}

func main() {
	root := "/Users/hubert_b/GolandProjects/image_averager/hscf"
	files, err := ioutil.ReadDir(root)
	if err != nil {
		panic(err)
	}
	width, height, err := getImageDimensions(filepath.Join(root, files[0].Name()))

	ingestChannel := make(chan image.Image, INGEST_BUFFER)
	outputChannel := make(chan averageImage)
	for i := 0; i < SUM_WORKERS; i++ {
		go processImages(ingestChannel, outputChannel, width, height)
	}

	for _, file := range files {
		img, err := getImageFromFilePath(filepath.Join(root, file.Name()))
		if err != nil {
			fmt.Println(err)
		}
		ingestChannel <- img
	}
	close(ingestChannel)

	finalImg := newAveragedImage(width, height)
	for workers := 0; workers < SUM_WORKERS; workers++ {
		img := <-outputChannel
		for c := 0; c < 3; c++ {
			for w := 0; w < width; w++ {
				for h := 0; h < height; h++ {
					finalImg.pixels[c][w][h] += img.pixels[c][w][h]
				}
			}
		}
		finalImg.imagesSummed += img.imagesSummed
	}
	img := finalImg.toImage()
	f, err := os.Create("/Users/hubert_b/GolandProjects/image_averager/avg.jpg")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	err = jpeg.Encode(f, img, nil)
	if err != nil {
		panic(err)
	}

}

func getImageDimensions(path string) (width int, height int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(bufio.NewReader(f))
	width = config.Width
	height = config.Height
	return
}

func processImages(imgs <-chan image.Image, outputs chan<- averageImage, width int, height int) {
	averaged := newAveragedImage(width, height)
	for img := range imgs {
		for w := 0; w < width; w++ {
			for h := 0; h < height; h++ {
				pixel := img.At(w, h)
				r, g, b, _ := pixel.RGBA()
				averaged.pixels[0][w][h] += r
				averaged.pixels[1][w][h] += g
				averaged.pixels[2][w][h] += b
			}
		}
		averaged.imagesSummed++
	}
	outputs <- averaged
}

func getImageFromFilePath(filePath string) (image.Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}
