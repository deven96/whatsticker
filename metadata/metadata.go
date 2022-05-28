package metadata

import (
	"fmt"
	"os/exec"
)

const rawFile = "metadata/raw.exif"

// Exif creates the exif metadata file and writes it to the image
type Exif struct {
	// image to impose exif file on
	TargetImage string
}

// Write : writes the .exif onto the TargetImage
func (e Exif) Write() {
	commandString := fmt.Sprintf("webpmux -set exif %s %s -o %s", rawFile, e.TargetImage, e.TargetImage)
	fmt.Println(commandString)
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Failed to set webp metadata", err)
	}
}

// GenerateMetadata : Takes ConvertedPath, generates a TargetFile exif and appends that exif metadata to ConvertedPath
func GenerateMetadata(ConvertedPath string) {
	converter := Exif{
		TargetImage: ConvertedPath,
	}
	converter.Write()
}
