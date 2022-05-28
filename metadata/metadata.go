package metadata

import (
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

const rawFile = "metadata/raw.exif.tpl"

// Exif creates the exif metadata file and writes it to the image
type Exif struct {
	// target exif file to create
	TargetFile string
	// image to impose exif file on
	TargetImage string

	// Values to write into the raw exif file
	AndroidAppStoreLink  string
	IOSAppStoreLink      string
	StickerPackPublisher string
	StickerPackName      string
	StickerPackID        string
}

// Generate : creates TargetFile .exif file specific to the image that is needed for the image
func (e Exif) Generate() error {
	target, err := os.Create(e.TargetFile)
	if err != nil {
		fmt.Println(err)
	}
	t, err := template.ParseFiles(rawFile)
	if err != nil {
		fmt.Println(err)
	}
	err = t.Execute(target, e)
	if err != nil {
		fmt.Printf("Error creating exif file %#v\n", err)
	}
	return err
}

// Write : writes the .exif onto the TargetImage
func (e Exif) Write() {
	commandString := fmt.Sprintf("webpmux -set exif %s %s -o %s", e.TargetFile, e.TargetImage, e.TargetImage)
	fmt.Println(commandString)
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Failed to set webp metadata", err)
	}
}

// CleanUp : deletes the generated .exif
func (e Exif) CleanUp() {
	os.Remove(e.TargetFile)
}

// GenerateMetadata : Takes ConvertedPath, generates a TargetFile exif and appends that exif metadata to ConvertedPath
func GenerateMetadata(ConvertedPath, TargetFile string) {
	githubLinkEscaped := `https:\/\/github.com\/deven96\/whatsticker`
	converter := Exif{
		TargetFile:  TargetFile,
		TargetImage: ConvertedPath,

		AndroidAppStoreLink:  githubLinkEscaped,
		IOSAppStoreLink:      githubLinkEscaped,
		StickerPackPublisher: `github.com\/deven96`,
		StickerPackName:      "whatsticker",
		StickerPackID:        ConvertedPath,
	}
	defer converter.CleanUp()
	err := converter.Generate()
	if err == nil {
		converter.Write()
	}
}
