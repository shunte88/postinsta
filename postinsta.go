package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Davincible/goinsta/v3"
	"golang.org/x/image/webp"
)

type VE struct {
	Good   bool
	Offset int
}

var validExt = map[string]VE{
	".png":  VE{true, 4},
	".jpg":  VE{true, 0},
	".jpeg": VE{true, 0},
	".webp": VE{true, 5},
}

func uploadToInstagram(username, password, imagePath, caption string) error {

	insta := goinsta.New(username, password)
	if err := insta.Login(); err != nil {
		return err
	}
	defer insta.Logout()

	fmt.Println("Logged in as ......:", insta.Account.Username)
	fmt.Println("Processing File ...:", imagePath)
	file, err := os.Open(imagePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return err
	}
	defer file.Close()

	// Read image data into a byte slice
	imgData, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// upload the image
	fmt.Println("Caption reads .....:", caption)
	_, err = insta.Upload(
		&goinsta.UploadOptions{
			File: bytes.NewReader(imgData),
			// using meta content here to avoid HTML in Instagram
			Caption: caption,
		},
	)
	if err != nil {
		fmt.Println("upload of latest failed")
		return err
	}
	return err
}

func jpgEncode(img image.Image, output string) error {

	jFile, err := os.Create(output)
	if err != nil {
		return err
	}
	defer jFile.Close()

	return jpeg.Encode(jFile, img, nil)
}

func convertWebPToJpeg(input, output string) error {
	fmt.Println("Converting webp to jpg", input)
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()

	img, err := webp.Decode(f)
	if err != nil {
		return err
	}

	return jpgEncode(img, output)
}

func convertPngToJpeg(input, output string) error {
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return err
	}
	return jpgEncode(img, output)
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
		fmt.Println("INSTA_USERNAME not set")
		os.Exit(1)
	}
	password := os.Getenv("INSTA_PASS")
	tags := os.Getenv("INSTA_TAG")

	var latestFile fs.FileInfo
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
				if latestFile == nil || info.ModTime().After(latestFile.ModTime()) {
					latestFile = info
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	if latestFile != nil {
		offset := validExt[filepath.Ext(latestFile.Name())].Offset
		if offset != 0 {
			newName := latestFile.Name()[:len(latestFile.Name())-offset] + ".jpg" // remove the webp extension and add jpg
			newPath := filepath.Join(*folder, newName)
			switch offset {
			case 4:
				if err := convertPngToJpeg(filepath.Join(*folder, latestFile.Name()), newPath); err != nil {
					fmt.Println("Error converting png to jpg:", err)
					os.Exit(1)
				}
			case 5:
				if err := convertWebPToJpeg(filepath.Join(*folder, latestFile.Name()), newPath); err != nil {
					fmt.Println("Error converting webp to jpg:", err)
					os.Exit(1)
				}
			}
			if err := moveToHistory(filepath.Join(*folder, latestFile.Name()), filepath.Join(*folder, "history")); err != nil {
				fmt.Println("Error moving webp file to history:", err)
				os.Exit(1)
			}
			// update the latest file
			latestFile, err = os.Stat(newPath)
			if err != nil {
				fmt.Println("Error getting file info:", err)
				os.Exit(1)
			}
		}

		fmt.Println("Latest image, processing:", latestFile.Name())
		caption := latestFile.Name()[:len(latestFile.Name())-4]  // remove the extension
		caption = strings.Join(strings.Split(caption, "_"), " ") // replace _ with space

		if tags != "" {
			caption = caption + " #" + strings.Join(strings.Split(tags, `,`), " #")
		}
		fmt.Println(caption)
		if err := uploadToInstagram(username, password, filepath.Join(*folder, latestFile.Name()), caption); err != nil {
			fmt.Println("Error uploading to Instagram:", err)
			os.Exit(1)
		} else {
			fmt.Println("Uploaded to Instagram")
			// move the file to history
			if err := moveToHistory(filepath.Join(*folder, latestFile.Name()), filepath.Join(*folder, "history")); err != nil {
				fmt.Println("Error moving file to history:", err)
				os.Exit(1)
			}

		}

	} else {
		fmt.Println("No files found this iteration")
	}
}
