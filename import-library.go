package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

func ImportLibrary() {

	path := "/Users/Mau/Desktop/Library.xml"

	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	generateAlbumsAndArtists(lines)
}

func generateAlbumsAndArtists(lines []string) {

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "<key>Album</key>") {
			albumLine := strings.Split(line, "<key>Album</key><string>")
			albumName := strings.Split(albumLine[1], "</string>")[0]
			fmt.Print(albumName + "\n")
			continue
		}
	}
}
