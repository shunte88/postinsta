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

	"github.com/Davincible/goinsta/v3"
	"golang.org/x/image/webp"
)

var validExt = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

func uploadToInstagram(username, password, imagePath, caption string) error {
	insta := goinsta.New(username, password)
	if err := insta.Login(); err != nil {
		return err
	}
	defer insta.Logout()

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
	password := os.Getenv("INSTA_PASS")
	tags := os.Getenv("INSTA_TAG")

	// fmt.Println(username, password, tags)

	var latestFile fs.FileInfo
	err := filepath.WalkDir(*folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if validExt[filepath.Ext(info.Name())] {
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
		// if the file has a webp extension, convert it to jpg
		if filepath.Ext(latestFile.Name()) == ".webp" {
			newName := latestFile.Name()[:len(latestFile.Name())-5] + ".jpg" // remove the webp extension and add jpg
			newPath := filepath.Join(*folder, newName)
			if err := convertWebPToJpeg(filepath.Join(*folder, latestFile.Name()), newPath); err != nil {
				fmt.Println("Error converting webp to png:", err)
				os.Exit(1)
			}
			// move the webp file to history
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
		} else if filepath.Ext(latestFile.Name()) == ".png" {
			newName := latestFile.Name()[:len(latestFile.Name())-4] + ".jpg" // remove the webp extension and add jpg
			newPath := filepath.Join(*folder, newName)
			if err := convertPngToJpeg(filepath.Join(*folder, latestFile.Name()), newPath); err != nil {
				fmt.Println("Error converting webp to jpg:", err)
				os.Exit(1)
			}
			// move the png file to history
			if err := moveToHistory(filepath.Join(*folder, latestFile.Name()), filepath.Join(*folder, "history")); err != nil {
				fmt.Println("Error moving png file to history:", err)
				os.Exit(1)
			}
			// update the latest file
			latestFile, err = os.Stat(newPath)
			if err != nil {
				fmt.Println("Error getting file info:", err)
				os.Exit(1)
			}
		}

		fmt.Println("Latest file:", latestFile.Name())
		if err := uploadToInstagram(username, password, filepath.Join(*folder, latestFile.Name()), tags); err != nil {
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
		fmt.Println("No files found")
	}
}
