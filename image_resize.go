package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/disintegration/imaging"
	log "github.com/sirupsen/logrus"
)

func resizeImage(body io.Reader, imageType string, newDimensions Dimensions) ([]byte, error) {
	if imageType != "image/jpeg" && imageType != "image/png" {
		log.WithField("imageType", imageType).Error("Unsupported image type")
		return nil, errors.New(fmt.Sprintf("Unsupported image type %v", imageType))
	}
	img, _, err := image.Decode(body)
	if err != nil {
		return nil, err
	}
	resImg := imaging.Resize(img, newDimensions.width, newDimensions.height, imaging.Lanczos)
	var imgBytes []byte
	if imageType == "image/jpeg" {
		imgBytes, err = jpegToBytes(resImg)
		if err != nil {
			return nil, err
		}
	} else if imageType == "image/png" {
		imgBytes, err = pngToBytes(resImg)
		if err != nil {
			return nil, err
		}
	}
	return imgBytes, nil
}

func jpegToBytes(img image.Image) ([]byte, error) {
	var opt jpeg.Options
	opt.Quality = 100

	buff := bytes.NewBuffer(nil)
	err := jpeg.Encode(buff, img, &opt)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func pngToBytes(img image.Image) ([]byte, error) {
	buff := bytes.NewBuffer(nil)
	err := png.Encode(buff, img)
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}
