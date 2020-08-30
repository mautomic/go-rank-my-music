package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const baseUrl = "https://rateyourmusic.com/release/album/"
const minWait = 60
const maxWait = 120

func main() {

	rand.Seed(time.Now().UnixNano())
	var ctx = context.Background()
	redisClient := createRedisClient(ctx)

	reg, _ := regexp.Compile("[^-/a-zA-Z0-9]+")

	albums := ImportLibrary()
	for i := 0; i < len(albums); i++ {
		albums[i].albumName = formatAlbumName(albums[i].albumName, reg)
		albums[i].artistName = formatArtistName(albums[i].artistName, reg)
	}

	for i := 0; i < len(albums); i++ {

		// define url to get rating from
		album := albums[i]
		url := baseUrl + album.artistName + "/" + album.albumName
		log.Print(url)

		// get html/js from url
		resp, err := http.Get(url)
		if err != nil {
			log.Print("Error getting response from request", err)
		}

		// read all bytes from the response body and convert to a string
		html, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Print("Error reading response body", err)
		}
		htmlString := string(html)

		// close response body
		err = resp.Body.Close()
		if err != nil {
			log.Print("Error closing response body", err)
		}

		avgRating, numRatings := parseString(htmlString)

		if avgRating != "" {
			fmt.Printf(album.albumName + " from " + album.artistName +
				" has avg of " + avgRating + " from " + numRatings + " reviews")

			// publish key/values to redis
			err2 := redisClient.Set(ctx, album.albumName, avgRating, 0).Err()
			if err2 != nil {
				log.Print("Error publishing key/value to redis", err2)
			}
		} else {
			log.Print("Couldn't find " + album.albumName + " by " + album.artistName + " on rateyourmusic")
		}

		// sleep thread to not get rate limited by rateyourmusic
		randomVal := rand.Intn(maxWait-minWait+1) + minWait
		time.Sleep(time.Duration(randomVal) * time.Second)
	}
}

func parseString(html string) (string, string) {

	// split down html to just get rating
	var avgRating string
	var numRatings string

	if strings.Contains(html, "avg_rating") {
		htmlArray := strings.Split(html, "avg_rating")
		avgRating = strings.TrimSpace(strings.Split(htmlArray[1], "</span>")[0][3:])

		htmlArray = strings.Split(html, "num_ratings")
		numRatingsHtmlTag := strings.Split(htmlArray[1], "</span>")[0]
		numRatings = strings.TrimSpace(strings.Split(numRatingsHtmlTag, "<span >")[1])
	}
	return avgRating, numRatings
}

func createRedisClient(ctx context.Context) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Cannot connect to Redis...exiting", err)
	}
	log.Print("Connected to Redis instance")
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
