package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
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

	fmt.Print("There are ")
	fmt.Print(len(lines))
	fmt.Print(" lines in library xml file\n")
}
