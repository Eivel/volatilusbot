package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"

	"gopkg.in/telegram-bot-api.v4"
)

type result struct {
	Filename string
	URL      string
}

func main() {
	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable host=%s port=%s",
		os.Getenv("DB_USERNAME"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	db, err := sql.Open("postgres", dbinfo)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "could not connect to the RDS"))
	}
	defer db.Close()

	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Fatalln(errors.Wrap(err, "could not initialize bot instance"))
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "could not initialize the update channel"))
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

	if !hasPermissions(db, query.From.ID) {
		return
	}

	var results []result

	queryArgs := convertToLowerCase(strings.Split(query.Query, " "))

	rows, err := db.Query("SELECT filename, url FROM volly_assets WHERE tags @> '{" + strings.Join(queryArgs, ", ") + "}'")
	if err != nil {
		log.Println(errors.Wrap(err, "could not complete the query for links"))
		return
	}

	for rows.Next() {
		var filename string
		var url string
		err = rows.Scan(&filename, &url)
		if err != nil {
			log.Println(errors.Wrap(err, "could not read one or more link rows"))
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

func hasPermissions(db *sql.DB, userID int) bool {
	stringifiedID := strconv.Itoa(userID)
	rows, err := db.Query(fmt.Sprintf("SELECT DISTINCT users.telegram_id FROM users JOIN messages ON messages.sender_id = users.id JOIN chats ON chats.id = messages.chat_id WHERE chats.telegram_id = %s;", os.Getenv("DRACONIS_ID")))
	if err != nil {
		log.Println(errors.Wrap(err, "could not complete the query for permissions"))
		return false
	}

	for rows.Next() {
		var telegramID string
		err = rows.Scan(&telegramID)
		if err != nil {
			log.Println(errors.Wrap(err, "could not read one or more telegram ids"))
			continue
		}
		if stringifiedID == telegramID {
			return true
		}
	}
	log.Println("Permissions not found for userID: ", stringifiedID)
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
