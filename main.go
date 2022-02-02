package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const INGEST_BUFFER = 256
const INGEST_IMAGE_BUFFER = 256

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
	rootPtr := flag.String("images-source", "", "directory from which to average images")
	targetPtr := flag.String("target-image", "", "where to save averaged images")
	flag.Parse()
	root := *rootPtr

	workers := runtime.NumCPU()

	files, err := ioutil.ReadDir(root)
	if err != nil {
		panic(err)
	}
	width, height, err := getImageDimensions(filepath.Join(root, files[0].Name()))

	filesChannel := make(chan string, INGEST_IMAGE_BUFFER)
	ingestChannel := make(chan image.Image, INGEST_BUFFER)
	outputChannel := make(chan averageImage, workers)
	var processWg sync.WaitGroup
	processWg.Add(workers)

	for i := 0; i < workers; i++ {
		go processImages(ingestChannel, outputChannel, width, height, &processWg)
	}

	var loaderWg sync.WaitGroup
	loaderWg.Add(workers)

	for i := 0; i < workers; i++ {
		go loadImages(filesChannel, ingestChannel, &loaderWg)
	}

	bar := progressbar.Default(int64(len(files)))

	for _, file := range files {
		bar.Add(1)
		filesChannel <- filepath.Join(root, file.Name())
	}
	close(filesChannel)
	loaderWg.Wait()
	close(ingestChannel)
	processWg.Wait()

	finalImg := newAveragedImage(width, height)
	for _workers := 0; _workers < workers; _workers++ {
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
	f, err := os.Create(*targetPtr)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	err = jpeg.Encode(f, img, nil)
	if err != nil {
		panic(err)
	}

}

func loadImages(files <-chan string, images chan<- image.Image, wg *sync.WaitGroup) {
	for file := range files {
		img, err := getImageFromFilePath(file)
		if err != nil {
			fmt.Println(err)
		}
		images <- img
	}
	wg.Done()
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

func processImages(imgs <-chan image.Image, outputs chan<- averageImage, width int, height int, s *sync.WaitGroup) {
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
	s.Done()
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
