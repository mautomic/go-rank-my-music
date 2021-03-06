package main

import (
	"bufio"
	"github.com/emirpasic/gods/sets/hashset"
	"log"
	"os"
	"strings"
)

const libraryFile = "Library.xml"

type album struct {
	albumName  string
	artistName string
}

// creates a new album provided a name and the artist
func newAlbum(name string, artist string) *album {
	a := album{albumName: name, artistName: artist}
	return &a
}

// reads the contents of the provided itunes xml file
func ImportLibrary() []album {

	file, err := os.Open(libraryFile)
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
	return generateAlbums(lines)
}

// generates an array of album structs from the xml lines
func generateAlbums(lines []string) []album {

	var albums []album
	albumSet := hashset.New()
	var artistNameHolder [1]string

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if strings.Contains(line, "<key>Artist</key>") {
			artistLine := strings.Split(line, "<key>Artist</key><string>")
			artistName := strings.Split(artistLine[1], "</string>")[0]
			artistNameHolder[0] = artistName
			continue
		}

		if strings.Contains(line, "<key>Album</key>") {
			albumLine := strings.Split(line, "<key>Album</key><string>")
			albumName := strings.Split(albumLine[1], "</string>")[0]

			if !albumSet.Contains(albumName) {
				albumSet.Add(albumName)
				albums = append(albums, *newAlbum(albumName, artistNameHolder[0]))
			}
			continue
		}
	}
	return albums
}
