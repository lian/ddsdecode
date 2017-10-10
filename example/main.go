package main

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/lian/ddsdecode"
)

func main() {
	file, err := os.Open("test.dds")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	tex, err := ddsdecode.Decode(file)
	if err != nil {
		fmt.Println("decode errror", err)
	}

	fmt.Println("decoded", tex.Width, tex.Height, len(tex.Data))

	image := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{tex.Width, tex.Height}})
	image.Pix = tex.Data

	out, _ := os.Create("test.png")
	png.Encode(out, image)
}
