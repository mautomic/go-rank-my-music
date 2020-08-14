package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func main() {

	ImportLibrary()

	artist := "jessie-ware"
	album := "whats-your-pleasure"

	// define url to get rating from
	url := "https://rateyourmusic.com/release/album/" + artist + "/" + album

	// get html/js from url
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	// close response body
	defer resp.Body.Close()

	// read all bytes from the response body and convert to a string
	html, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	htmlString := string(html)

	// split down html to just get rating
	htmlArray := strings.Split(htmlString, "avg_rating")
	rating := strings.TrimSpace(strings.Split(htmlArray[1], "</span>")[0][3:])

	fmt.Printf(rating)
}
