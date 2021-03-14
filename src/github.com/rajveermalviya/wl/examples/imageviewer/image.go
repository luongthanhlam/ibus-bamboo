package main

import (
	"bufio"
	"image"
	"image/draw"
	"log"
	"os"

	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

func imageFromFile(filePath string) (image.Image, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	img, _, err := image.Decode(br)
	if err != nil {
		return nil, err
	}

	rgbaImage, ok := img.(*image.RGBA)
	if !ok {
		// Convert to RGBA if not already RGBA
		rect := img.Bounds()
		rgbaImage = image.NewRGBA(rect)
		draw.Draw(rgbaImage, rect, img, rect.Min, draw.Src)
	}

	return rgbaImage, nil
}

func copyImage(src image.Image) image.Image {
	switch t := src.(type) {
	case *image.RGBA:
		pix := make([]uint8, len(t.Pix))
		copy(pix, t.Pix)
		return &image.RGBA{
			Pix:    pix,
			Stride: t.Stride,
			Rect:   t.Rect,
		}

	default:
		log.Fatalf("unable to copy image of type: %T", t)
		return nil
	}
}
