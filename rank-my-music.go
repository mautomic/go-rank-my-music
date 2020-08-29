package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"io/ioutil"
	"net/http"
	"strings"
)

func main() {

	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	pong, err := rdb.Ping(ctx).Result()
	fmt.Println(pong, err)

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
