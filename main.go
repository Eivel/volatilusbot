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

	"gopkg.in/telegram-bot-api.v4"

	_ "github.com/lib/pq"

	"github.com/joho/godotenv"
)

type permissions struct {
	Nicknames []string
}

type result struct {
	Filename string
	URL      string
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

	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatalln(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Println(err)
	}

	log.Println("Masz krowę?")

	for update := range updates {
		go processUpdate(bot, db, update)
	}
}

func processUpdate(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update) {
	if update.InlineQuery == nil {
		return
	}
	query := update.InlineQuery
	log.Println("--- new query ---")
	log.Println("from:", query.From.UserName)
	log.Println("text:", query.Query)

	offset := query.Offset
	if offset == "" {
		offset = "0"
	}
	perPage := 50

	if !hasPermissions(query.From.UserName) {
		return
	}

	var results []result

	queryArgs := convertToLowerCase(strings.Split(query.Query, " "))
	rows, err := db.Query("SELECT filename, url FROM volly_assets WHERE tags @> '{" + strings.Join(queryArgs, ", ") + "}'")
	if err != nil {
		log.Println(err)
		return
	}
	for rows.Next() {
		var filename string
		var url string
		err = rows.Scan(&filename, &url)
		if err != nil {
			log.Println(err)
			continue
		}
		results = append(results, result{Filename: filename, URL: url})
	}

	lowerLimit, upperLimit, offset := calculateLimits(perPage, offset, len(results))

	images := make([]interface{}, 0)

	if len(results) > 0 {
		log.Printf("lower: %v, upper: %v, length: %v", lowerLimit, upperLimit, len(results))
		for iter, result := range results[lowerLimit:upperLimit] {
			splitted := strings.Split(strings.Split(strings.Split(result.Filename, "_")[1], ".")[0], "x")
			splitted = strings.Split(result.Filename, ".")
			extension := splitted[len(splitted)-1]
			if extension == "mp4" {
				gif := tgbotapi.InlineQueryResultMPEG4GIF{
					Type:                "mpeg4_gif",
					ID:                  strconv.Itoa(iter),
					Title:               result.Filename,
					URL:                 result.URL,
					ThumbURL:            result.URL,
					InputMessageContent: tgbotapi.InputTextMessageContent{Text: result.URL},
				}
				images = append(images, gif)
			} else {
				photo := tgbotapi.InlineQueryResultPhoto{
					Type:     "photo",
					ID:       strconv.Itoa(iter),
					Title:    result.Filename,
					URL:      result.URL,
					ThumbURL: result.URL,
					InputMessageContent: tgbotapi.InputTextMessageContent{
						Text: result.URL,
						DisableWebPagePreview: false,
					},
				}
				images = append(images, photo)
			}
		}
	}

	response := tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		Results:       images,
		IsPersonal:    true,
		NextOffset:    offset,
		CacheTime:     1,
	}

	apiResponse, err := bot.AnswerInlineQuery(response)
	if err != nil {
		log.Println("Failed to respond to query:", err)
	}
	if !apiResponse.Ok {
		log.Println("API error:", err)
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
	upper := convertedOffset*perPage + perPage
	if upper >= length {
		return lower, length, ""
	} else if upper == (length - 1) {
		return lower, upper, ""
	} else {
		return lower, upper, strconv.Itoa(convertedOffset + 1)
	}
}

func convertToLowerCase(args []string) []string {
	out := make([]string, len(args))
	for i, el := range args {
		out[i] = strings.ToLower(el)
	}
	return out
}
