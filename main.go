package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tucnak/telebot"

	_ "github.com/lib/pq"

	"github.com/joho/godotenv"
)

var bot *telebot.Bot

type permissions struct {
	Nicknames []string
}

type result struct {
	Filename string
	Url      string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%s",
		os.Getenv("DB_USERNAME"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	db, _ := sql.Open("postgres", dbinfo)
	defer db.Close()

	bot, err = telebot.NewBot(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatalln(err)
	}

	bot.Queries = make(chan telebot.Query, 1000)

	go queries(db)
	log.Println("Masz krowę?")

	bot.Start(1 * time.Second)
}

func queries(db *sql.DB) {
	for query := range bot.Queries {
		log.Println("--- new query ---")
		log.Println("from:", query.From.Username)
		log.Println("text:", query.Text)
		log.Printf("%+v", query)
		offset := query.Offset
		if offset == "" {
			offset = "0"
		}
		perPage := 50

		if !hasPermissions(query.From.Username) {
			continue
		}

		var results []result

		queryArgs := strings.Split(query.Text, " ")
		fmt.Println("# Querying")
		rows, err := db.Query("SELECT filename, url FROM volly_assets WHERE tags @> '{" + strings.Join(queryArgs, ", ") + "}'")
		if err != nil {
			log.Println(err)
			continue
		}
		for rows.Next() {
			var filename string
			var url string
			err = rows.Scan(&filename, &url)
			if err != nil {
				log.Println(err)
				continue
			}
			results = append(results, result{Filename: filename, Url: url})
		}

		lowerLimit, upperLimit, offset := calculateLimits(perPage, offset, len(results))

		images := []telebot.InlineQueryResult{}
		if len(results) > 0 {
			log.Printf("lower: %v, upper: %v, length: %v", lowerLimit, upperLimit, len(results))
			for _, result := range results[lowerLimit:upperLimit] {
				splitted := strings.Split(strings.Split(strings.Split(result.Filename, "_")[1], ".")[0], "x")
				splitted = strings.Split(result.Filename, ".")
				extension := splitted[len(splitted)-1]
				if extension == "mp4" {
					gif := &telebot.InlineQueryResultMpeg4Gif{
						Title:    result.Filename,
						URL:      result.Url,
						ThumbURL: result.Url,
					}
					images = append(images, gif)
				} else {
					photo := &telebot.InlineQueryResultPhoto{
						Title:    result.Filename,
						PhotoURL: result.Url,
						ThumbURL: result.Url,
						InputMessageContent: &telebot.InputTextMessageContent{
							Text:           result.Url,
							DisablePreview: false,
						},
					}
					images = append(images, photo)
				}
			}
		}

		response := telebot.QueryResponse{
			Results:    images,
			IsPersonal: true,
			NextOffset: offset,
			CacheTime:  1,
		}

		if err := bot.AnswerInlineQuery(&query, &response); err != nil {
			log.Println("Failed to respond to query:", err)
		}
	}
}

func hasPermissions(username string) bool {
	file, e := ioutil.ReadFile("./.permissions.json")
	if e != nil {
		log.Printf("File error: %v\n", e)
		return false
	}

	var permissions permissions
	json.Unmarshal(file, &permissions)
	for _, el := range permissions.Nicknames {
		if el == username {
			return true
		}
	}
	return false
}

func calculateLimits(perPage int, offset string, length int) (int, int, string) {
	convertedOffset, err := strconv.Atoi(offset)
	if err != nil {
		log.Println(err)
		convertedOffset = 0
	}
	lower := convertedOffset * perPage
	upper := convertedOffset*perPage + perPage - 1
	if upper >= length {
		return lower, length - 1, ""
	} else if upper == (length - 1) {
		return lower, upper, ""
	} else {
		return lower, upper, strconv.Itoa(convertedOffset + 1)
	}
}
