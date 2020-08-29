package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

func main() {

	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
	if strings.EqualFold(pong, "PONG") {
		fmt.Println("Connected to Redis instance")
	}

	reg, err := regexp.Compile("[^-/a-zA-Z0-9]+")
	if err != nil {
		panic(err)
	}

	albums := ImportLibrary()
	for i := 0; i < len(albums); i++ {
		album := albums[i]
		albumName := strings.TrimSpace(strings.ToLower(album.albumName))
		albumName = strings.ReplaceAll(albumName, " ep", "")
		albumName = strings.ReplaceAll(albumName, "deluxe", "")
		albumName = strings.ReplaceAll(albumName, "single", "")
		albumName = strings.ReplaceAll(albumName, "remastered", "")
		albumName = strings.ReplaceAll(albumName, "edition", "")
		albumName = strings.ReplaceAll(albumName, "expanded", "")
		albumName = strings.ReplaceAll(albumName, "version", "")
		albumName = strings.ReplaceAll(albumName, "38", "and")
		albumName = strings.ReplaceAll(albumName, " ", "-")
		albumName = strings.ReplaceAll(albumName, "/", "_")
		albumName = reg.ReplaceAllString(albumName, "")
		albumName = strings.Trim(albumName, "-")

		artistName := strings.TrimSpace(strings.ToLower(album.artistName))
		artistName = strings.ReplaceAll(artistName, " ", "-")
		artistName = strings.ReplaceAll(artistName, "/", "_")
		artistName = strings.ReplaceAll(artistName, "38", "and")
		artistName = reg.ReplaceAllString(artistName, "")
		artistName = strings.Trim(artistName, "-")

		fmt.Println(albumName + " " + artistName)
	}

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
	avgRating := strings.TrimSpace(strings.Split(htmlArray[1], "</span>")[0][3:])

	htmlArray = strings.Split(htmlString, "num_ratings")
	numRatingsHtmlTag := strings.Split(htmlArray[1], "</span>")[0]
	numRatings := strings.TrimSpace(strings.Split(numRatingsHtmlTag, "<span >")[1])

	fmt.Printf(album + " from " + artist + " has avg of " + avgRating + " from " + numRatings + " reviews")

	err2 := rdb.Set(ctx, album, avgRating, 0).Err()
	if err2 != nil {
		panic(err2)
	}
}
