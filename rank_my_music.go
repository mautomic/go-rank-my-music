package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const BASE_URL = "https://rateyourmusic.com/release/album/"
const MIN_WAIT = 150
const MAX_WAIT = 380
const REDIS_ALBUM_KEY = "ALBUM:"
const REDIS_MISSING_ALBUMS_KEY = "MISSING_ALBUMS"
const AVG_RATING = "avg_rating"
const NUM_RATINGS = "num_ratings"

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
	reg, _ := regexp.Compile("[^-_/a-zA-Z0-9]+")

	albums := ImportLibrary()

	// get album names that have not been found on rateyourmusic previously from redis
	missingAlbums := redisClient.SMembers(ctx, REDIS_MISSING_ALBUMS_KEY).Val()
	log.Println("# of albums is " + strconv.Itoa(len(albums)))

	// begin scraping on a different goroutine
	go scrapeDataFromRYM(albums, ctx, redisClient, missingAlbums, reg)

	// setup a webserver to retrieve album information
	r := gin.Default()
	r.GET("/rating/:albumName", func(c *gin.Context) {
		albumName := c.Param("albumName")
		albumNameRedisKey := REDIS_ALBUM_KEY + albumName
		data := redisClient.SMembers(ctx, albumNameRedisKey).Val()
		if len(data) < 2 {
			c.String(http.StatusOK, "Could not find album %s in cache", albumName)
		} else {
			rating := data[0]
			reviews := data[1]
			c.String(http.StatusOK, "%s has a rating of %s with %s reviews", albumName, rating, reviews)
		}
	})
	r.Run() // listen and serve on 0.0.0.0:8080

}

// this function contains core logic to begin scraping RYM at some random interval
// and publishes results to redis
func scrapeDataFromRYM(albums []album, ctx context.Context, redisClient *redis.Client,
	missingAlbums []string, reg *regexp.Regexp) {

	for i := 0; i < len(albums); i++ {
		/*
			this piece of code will format all albums and artist names for rateyourmusic queries
			they will typically be all lowercase, have dashes in between words, and not have special characters
			eg. Whats Your Pleasure by Jessie Ware -> 'whats-your-pleasure' and 'jessie-ware'
			url will look like: https://rateyourmusic.com/release/album/jessie-ware/whats-your-pleasure
		*/
		albums[i].albumName = formatAlbumName(albums[i].albumName, reg)
		albums[i].artistName = formatArtistName(albums[i].artistName, reg)

		// skip album if we already have data for it in redis
		value := redisClient.SMembers(ctx, REDIS_ALBUM_KEY+albums[i].albumName).Val()
		if len(value) > 0 {
			continue
		}

		// skip albums that we know cannot be found during album iteration
		if contains(missingAlbums, albums[i].albumName) {
			continue
		}

		// create url to get rating information from
		url := BASE_URL + albums[i].artistName + "/" + albums[i].albumName

		// get html/js from url
		resp, err := http.Get(url)
		if err != nil {
			log.Println("Error getting response from request", err)
		}

		// read all bytes from the response body and convert to a string
		html, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("Error reading response body", err)
		}
		htmlString := string(html)

		// close response body first before parsing response html string
		err = resp.Body.Close()
		if err != nil {
			log.Println("Error closing response body", err)
		}

		avgRating, numRatings := getRatingsFromResponseString(htmlString)

		// if a rating was actually set, then a successful response was received AND parsed
		if avgRating != "" {
			fmt.Println(albums[i].albumName + " from " + albums[i].artistName +
				" has avg of " + avgRating + " from " + numRatings + " reviews")
			err = publishRatings(ctx, redisClient, albums[i].albumName, avgRating, numRatings)
			if err != nil {
				log.Println("Error publishing "+albums[i].albumName+" ratings to redis", err)
			}
		} else {
			log.Println("Couldn't find (" + strconv.Itoa(i+1) + ") " + albums[i].albumName +
				" by " + albums[i].artistName + " on rateyourmusic")
			// add missing albums to an array (indicates bad url) and publish to redis
			err := publishMissingAlbum(ctx, redisClient, albums[i].albumName)
			if err != nil {
				log.Println("Error publishing "+albums[i].albumName+" to missing albums set in redis", err)
			}
		}

		// sleep thread for a random time period before continuing queries
		// this is to avoid getting this IP blocked by rateyourmusic
		randomVal := rand.Intn(MAX_WAIT-MIN_WAIT+1) + MIN_WAIT
		time.Sleep(time.Duration(randomVal) * time.Second)
	}
}

// gets the average rating and number of ratings from the response html string
func getRatingsFromResponseString(html string) (string, string) {

	var avgRating string
	var numRatings string

	// if avg_rating is not in the response html, then most likely the album was not found
	// and empty strings can be returned
	if strings.Contains(html, AVG_RATING) {
		htmlArray := strings.Split(html, AVG_RATING)
		avgRating = strings.TrimSpace(strings.Split(htmlArray[1], "</span>")[0][3:])

		htmlArray = strings.Split(html, NUM_RATINGS)
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
	log.Println("Connected to Redis instance")
	return redisClient
}

// publish avgRating and numRatings to redis for later analysis
// (key, value) = (album name, [avgRating, numRatings])
func publishRatings(ctx context.Context, client *redis.Client, albumName string, avgRating string, numRatings string) error {
	err1 := client.SAdd(ctx, REDIS_ALBUM_KEY+albumName, avgRating).Err()
	if err1 != nil {
		return err1
	}
	err2 := client.SAdd(ctx, REDIS_ALBUM_KEY+albumName, numRatings).Err()
	if err2 != nil {
		return err2
	}
	return nil
}

// publish album name that was not found on rateyourmusic to redis. this is for keeping an
// index of albums that were unable to be parsed, and can be used to avoid wasting time
// trying to request previously not-found albums in case of a halt, or IP block. There are
// many reasons why a url may have not been found, which includes being difficult to parse,
// or the album metadata kept by itunes is a particular edition/compilation/single
func publishMissingAlbum(ctx context.Context, client *redis.Client, albumName string) error {
	err1 := client.SAdd(ctx, REDIS_MISSING_ALBUMS_KEY, albumName).Err()
	if err1 != nil {
		return err1
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

// utility function to check if slice contains the specified string
func contains(arr []string, element string) bool {
	for _, item := range arr {
		if item == element {
			return true
		}
	}
	return false
}
