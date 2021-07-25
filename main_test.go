package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateNameForResizedFile(t *testing.T) {
	var dimensions = Dimensions{
		width:  64,
		height: 64,
	}
	assert.Equal(t, "file__64x64.jpeg", createNameForResizedFile("file.jpeg", dimensions, "image/jpeg"))
	assert.Equal(t, "file__64x64.jpeg", createNameForResizedFile("file", dimensions, "image/jpeg"))
	assert.Equal(t, "file__64x64.png", createNameForResizedFile("file.png", dimensions, "image/png"))
	assert.Equal(t, "file__64x64.png", createNameForResizedFile("file", dimensions, "image/png"))
}
