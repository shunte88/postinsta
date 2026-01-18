package main

import (
	"bytes"
	"flag"
	"image"
	"image/jpeg"
	"log"

	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Davincible/goinsta/v3"
	_ "golang.org/x/image/webp"
)

type VE struct {
	Good      bool
	Supported bool
	Offset    int
}

var validExt = map[string]VE{
	".png":  VE{true, false, 4}, // not supported by Instagram
	".jpg":  VE{true, true, 0},
	".jpeg": VE{true, true, 0},
	".gif":  VE{true, false, 0}, // not supported by Instagram???
	".webp": VE{true, false, 5}, // not supported by Instagram
}

func uploadToInstagram(username, password, imagePath, caption string) error {

	insta := goinsta.New(username, password)
	if err := insta.Login(); err != nil {
		return err
	}
	defer insta.Logout()

	log.Println("Logged in as ......:", insta.Account.Username)
	log.Println("Processing File ...:", imagePath)
	file, err := os.Open(imagePath)
	if err != nil {
		log.Println("Error opening file:", err)
		return err
	}
	defer file.Close()

	// Read image data into a byte slice
	imgData, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// upload the image
	log.Println("Caption reads .....:", caption)
	_, err = insta.Upload(
		&goinsta.UploadOptions{
			File: bytes.NewReader(imgData),
			// using meta content here to avoid HTML in Instagram
			Caption: caption,
		},
	)
	if err != nil {
		log.Println("upload of latest failed")
		return err
	}
	return err
}

func convertImageToJpeg(input, output string) error {
	// image in...
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	// and pop jpg out
	jFile, err := os.Create(output)
	if err != nil {
		return err
	}
	defer jFile.Close()

	return jpeg.Encode(jFile, img, nil)
}

func moveToHistory(input, historyFolder string) error {
	// create folder as needed
	err := os.MkdirAll(historyFolder, os.ModePerm)
	if err != nil {
		return err
	}
	return os.Rename(input, filepath.Join(historyFolder, filepath.Base(input)))
}

func main() {

	folder := flag.String("folder", ".", "Folder to scan for images")
	flag.Parse()

	username := os.Getenv("INSTA_USERNAME")
	if username == "" {
		log.Println("INSTA_USERNAME not set")
		os.Exit(1)
	}
	password := os.Getenv("INSTA_PASS")
	tags := os.Getenv("INSTA_TAG")

	var targetFile fs.FileInfo
	err := filepath.WalkDir(*folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !strings.Contains(path, `/history`) {

			info, err := d.Info()
			if err != nil {
				return err
			}
			if validExt[filepath.Ext(info.Name())].Good {
				if targetFile == nil || info.ModTime().Before(targetFile.ModTime()) {
					targetFile = info
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

	if targetFile != nil {
		offset := validExt[filepath.Ext(targetFile.Name())].Offset
		if offset != 0 {
			newName := targetFile.Name()[:len(targetFile.Name())-offset] + ".jpg" // remove the webp extension and add jpg
			newPath := filepath.Join(*folder, newName)
			if err := convertImageToJpeg(filepath.Join(*folder, targetFile.Name()), newPath); err != nil {
				log.Println("Error converting image to jpg:", err)
				os.Exit(1)
			}

			if err := moveToHistory(filepath.Join(*folder, targetFile.Name()), filepath.Join(*folder, "history")); err != nil {
				log.Println("Error moving original format file to history:", err)
				os.Exit(1)
			}
			// update the target file
			targetFile, err = os.Stat(newPath)
			if err != nil {
				log.Println("Error getting file info:", err)
				os.Exit(1)
			}
		}

		log.Println("Todays image ......:", targetFile.Name())
		caption := targetFile.Name()[:len(targetFile.Name())-4]  // remove the extension
		caption = strings.Join(strings.Split(caption, "_"), " ") // replace _ with space

		if tags != "" {
			caption = caption + " #" + strings.Join(strings.Split(tags, `,`), " #")
		}

		if err := uploadToInstagram(username, password, filepath.Join(*folder, targetFile.Name()), caption); err != nil {
			log.Println("Error uploading to Instagram:", err)
			os.Exit(1)
		} else {
			log.Println("Upload to Instagram complete...")
			// move the file to history
			if err := moveToHistory(filepath.Join(*folder, targetFile.Name()), filepath.Join(*folder, "history")); err != nil {
				log.Println("Error moving file to history:", err)
				os.Exit(1)
			}

		}

	} else {
		log.Println("No files found this iteration")
	}
}
