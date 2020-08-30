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

/*
function that kicks off go-rank-my-music. music metadata is first imported from itunes via
an XML file, and then formatted in order to hit rateyourmusic.com's page one album at a time.
the average rating and number of ratings for an album are published to redis for later
retrieval for analytics. requests to grab the html web page of rateyourmusic will occur at
a periodic interval defined by minWait and maxWait, otherwise the IP sending these requests
will get blocked (user must go accept a CAPTCHA on the site before getting access back).
*/
func main() {

	// create a redis client
	var ctx = context.Background()
	redisClient := createRedisClient(ctx)

	// setup rand seed and regex
	rand.Seed(time.Now().UnixNano())
	reg, _ := regexp.Compile("[^-/a-zA-Z0-9]+")

	albums := ImportLibrary()

	for i := 0; i < len(albums); i++ {
		/*
			this piece of code will format all albums and artist names for rateyourmusic queries
			they will typically be all lowercase, have dashes in between words, and not have special characters
			eg. Whats Your Pleasure by Jessie Ware -> 'whats-your-pleasure' and 'jessie-ware'
			url will look like: https://rateyourmusic.com/release/album/jessie-ware/whats-your-pleasure
		*/
		albums[i].albumName = formatAlbumName(albums[i].albumName, reg)
		albums[i].artistName = formatArtistName(albums[i].artistName, reg)

		// create url to get rating information from
		url := baseUrl + albums[i].artistName + "/" + albums[i].albumName

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

		// close response body first before parsing response html string
		err = resp.Body.Close()
		if err != nil {
			log.Print("Error closing response body", err)
		}

		avgRating, numRatings := getRatingsFromResponseString(htmlString)

		// if a rating was actually set, then a successful response was received AND parsed
		if avgRating != "" {
			fmt.Printf(albums[i].albumName + " from " + albums[i].artistName +
				" has avg of " + avgRating + " from " + numRatings + " reviews")
			err := publish(ctx, redisClient, albums[i].albumName, avgRating, numRatings)
			if err != nil {
				log.Print("Error publishing "+albums[i].albumName+" to redis", err)
			}
		} else {
			log.Print("Couldn't find " + albums[i].albumName + " by " + albums[i].artistName + " on rateyourmusic")
		}

		// sleep thread for a random time period before continuing queries
		// this is to avoid getting this IP blocked by rateyourmusic
		randomVal := rand.Intn(maxWait-minWait+1) + minWait
		time.Sleep(time.Duration(randomVal) * time.Second)
	}
}

// gets the average rating and number of ratings from the response html string
func getRatingsFromResponseString(html string) (string, string) {

	var avgRating string
	var numRatings string

	// if avg_rating is not in the response html, then most likely the album was not found
	// and empty strings can be returned
	if strings.Contains(html, "avg_rating") {
		htmlArray := strings.Split(html, "avg_rating")
		avgRating = strings.TrimSpace(strings.Split(htmlArray[1], "</span>")[0][3:])

		htmlArray = strings.Split(html, "num_ratings")
		numRatingsHtmlTag := strings.Split(htmlArray[1], "</span>")[0]
		numRatings = strings.TrimSpace(strings.Split(numRatingsHtmlTag, "<span >")[1])
	}
	return avgRating, numRatings
}

// creates a redis client for publishing data
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

// publish avgRating and numRatings to redis for later analysis
// (key, value) = (album name, [avgRating, numRatings])
func publish(ctx context.Context, client *redis.Client, albumName string, avgRating string, numRatings string) error {
	err1 := client.SAdd(ctx, albumName, avgRating, 0).Err()
	if err1 != nil {
		return err1
	}
	err2 := client.SAdd(ctx, albumName, numRatings, 0).Err()
	if err2 != nil {
		return err2
	}
	return nil
}

// formats the album name to comply with rateyourmusic's urls
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

// formats the artist name to comply with rateyourmusic's urls
func formatArtistName(artistName string, regex *regexp.Regexp) string {
	artistName = strings.TrimSpace(strings.ToLower(artistName))
	artistName = strings.ReplaceAll(artistName, " ", "-")
	artistName = strings.ReplaceAll(artistName, "/", "_")
	artistName = strings.ReplaceAll(artistName, "38", "and")
	artistName = regex.ReplaceAllString(artistName, "")
	return strings.Trim(artistName, "-")
}
