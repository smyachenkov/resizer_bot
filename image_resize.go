package main

import (
	"bytes"
	"github.com/disintegration/imaging"
	log "github.com/sirupsen/logrus"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	"io"
)
import _ "image/png"

func resizeImage(body io.ReadCloser, newDimensions Dimensions) ([]byte, error) {
	img, _, err := image.Decode(body)
	if err != nil {
		return nil, err
	}
	resImg := imaging.Resize(img, newDimensions.width, newDimensions.height, imaging.Lanczos)
	imgBytes := imgToBytes(resImg)
	return imgBytes, nil
}

func imgToBytes(img image.Image) []byte {
	var opt jpeg.Options
	opt.Quality = 100

	buff := bytes.NewBuffer(nil)
	err := jpeg.Encode(buff, img, &opt)
	if err != nil {
		log.Fatal(err)
	}

	return buff.Bytes()
}
