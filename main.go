package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tucnak/telebot"

	"github.com/joho/godotenv"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var bot *telebot.Bot

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	url := fmt.Sprintf(
		"mongodb://%s:%s@%s/%s",
		os.Getenv("DB_USERNAME"),
		os.Getenv("DB_USERPASS"),
		os.Getenv("DB_URL"),
		os.Getenv("DB_NAME"))
	session, err := mgo.Dial(url)
	if err != nil {
		log.Fatalln(err)
		return
	}
	fmt.Printf("Connected to %v!\n", session.LiveServers())

	coll := session.DB(os.Getenv("DB_NAME")).C(os.Getenv("COLL_NAME"))

	bot, err = telebot.NewBot(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatalln(err)
	}

	bot.Queries = make(chan telebot.Query, 1000)

	go queries(coll)
	log.Println("Masz krowę?")

	bot.Start(1 * time.Second)
}

func queries(coll *mgo.Collection) {
	for query := range bot.Queries {
		log.Println("--- new query ---")
		log.Println("from:", query.From.Username)
		log.Println("text:", query.Text)

		var results []struct {
			Filename string   `bson:"filename"`
			Link     string   `bson:"link"`
			Tags     []string `bson:"tags"`
		}

		queryArgs := strings.Split(query.Text, " ")
		if len(queryArgs) > 1 {
			iter := coll.Find(bson.M{"tags": bson.M{"$all": strings.Split(strings.ToLower(query.Text), " ")}}).Limit(50).Iter()
			err := iter.All(&results)
			if err != nil {
				log.Fatalln(err)
				return
			}
		} else {
			iter := coll.Find(bson.M{"tags": strings.ToLower(query.Text)}).Limit(50).Iter()
			err := iter.All(&results)
			if err != nil {
				log.Fatalln(err)
				return
			}
		}

		images := []telebot.InlineQueryResult{}
		for _, result := range results {
			splitted := strings.Split(strings.Split(strings.Split(result.Filename, "_")[1], ".")[0], "x")
			width, err := strconv.Atoi(splitted[0])
			height, err := strconv.Atoi(splitted[1])
			if err != nil {
				log.Fatalln(err)
				return
			}
			splitted = strings.Split(result.Filename, ".")
			extension := splitted[len(splitted)-1]
			if extension == "mp4" {
				gif := &telebot.InlineQueryResultMpeg4Gif{
					Title:    result.Filename,
					URL:      result.Link,
					Width:    width,
					Height:   height,
					ThumbURL: result.Link,
				}
				images = append(images, gif)
			} else {
				photo := &telebot.InlineQueryResultPhoto{
					Title:       result.Filename,
					PhotoURL:    result.Link,
					PhotoWidth:  width,
					PhotoHeight: height,
					ThumbURL:    result.Link,
					InputMessageContent: &telebot.InputTextMessageContent{
						Text:           result.Link,
						DisablePreview: false,
					},
				}
				images = append(images, photo)
			}
		}

		response := telebot.QueryResponse{
			Results:    images,
			IsPersonal: true,
		}

		if err := bot.AnswerInlineQuery(&query, &response); err != nil {
			log.Println("Failed to respond to query:", err)
		}
	}
}
