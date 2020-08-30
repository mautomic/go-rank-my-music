package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

const baseUrl = "https://rateyourmusic.com/release/album/"

func main() {

	var ctx = context.Background()
	redisClient := createRedisClient(ctx)

	reg, err := regexp.Compile("[^-/a-zA-Z0-9]+")

	albums := ImportLibrary()
	for i := 0; i < len(albums); i++ {
		albums[i].albumName = formatAlbumName(albums[i].albumName, reg)
		albums[i].artistName = formatArtistName(albums[i].artistName, reg)
	}

	album := albums[0]

	// define url to get rating from
	url := baseUrl + album.artistName + "/" + album.albumName
	log.Print(url)

	// get html/js from url
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
	// close response body
	defer resp.Body.Close()

	// read all bytes from the response body and convert to a string
	html, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
	htmlString := string(html)

	// split down html to just get rating
	htmlArray := strings.Split(htmlString, "avg_rating")
	avgRating := strings.TrimSpace(strings.Split(htmlArray[1], "</span>")[0][3:])

	htmlArray = strings.Split(htmlString, "num_ratings")
	numRatingsHtmlTag := strings.Split(htmlArray[1], "</span>")[0]
	numRatings := strings.TrimSpace(strings.Split(numRatingsHtmlTag, "<span >")[1])

	fmt.Printf(album.albumName + " from " + album.artistName +
		" has avg of " + avgRating + " from " + numRatings + " reviews")

	err2 := redisClient.Set(ctx, album.albumName, avgRating, 0).Err()
	if err2 != nil {
		log.Fatal(err2)
	}
}

func createRedisClient(ctx context.Context) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	pong, err := redisClient.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
	if strings.EqualFold(pong, "PONG") {
		log.Print("Connected to Redis instance")
	} else {
		log.Fatal("Cannot connect to Redis...exiting", err)
		os.Exit(3)
	}
	return redisClient
}

func formatAlbumName(albumName string, regex *regexp.Regexp) string {
	albumName = strings.TrimSpace(strings.ToLower(albumName))
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
	albumName = regex.ReplaceAllString(albumName, "")
	return strings.Trim(albumName, "-")
}

func formatArtistName(artistName string, regex *regexp.Regexp) string {
	artistName = strings.TrimSpace(strings.ToLower(artistName))
	artistName = strings.ReplaceAll(artistName, " ", "-")
	artistName = strings.ReplaceAll(artistName, "/", "_")
	artistName = strings.ReplaceAll(artistName, "38", "and")
	artistName = regex.ReplaceAllString(artistName, "")
	return strings.Trim(artistName, "-")
}
