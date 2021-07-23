package main

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Dimensions struct {
	width  int
	height int
}

func main() {

	log.SetOutput(os.Stdout)

	// heroku healthcheck
	go http.ListenAndServe(":"+os.Getenv("PORT"), nil)

	// map of chat_id to last image in it
	var lastChatImage = make(map[int64]string)

	botToken := checkEnvVariable("BOT_TOKEN")

	bot, err := tb.NewBot(tb.Settings{
		Token:  botToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	bot.Handle(tb.OnDocument, func(m *tb.Message) {
		// todo implement
	})

	bot.Handle(tb.OnPhoto, func(m *tb.Message) {
		chatId := m.Chat.ID
		fileId := m.Photo.FileID
		lastChatImage[chatId] = fileId

		log.WithFields(log.Fields{
			"fileId": strconv.FormatInt(chatId, 10),
			"chatId": fileId,
		}).Info("New image queued for chat")

		// request dimensions
		bot.Send(m.Sender, "Great\\! Now send me width and height, for example, `128x128`", tb.ModeMarkdownV2)
	})

	bot.Handle(tb.OnText, func(m *tb.Message) {
		// greeting
		if m.Text == "/start" {
			bot.Send(m.Sender, "Hello! I can resize images for you. Send me a file or an image")
			return
		}
		chatId := m.Chat.ID
		if imageFileId, ok := lastChatImage[chatId]; ok {
			// parse dimensions
			r := regexp.MustCompile("[(\\d+)x(\\d+)]+")
			matchedDimensions := r.FindAllStringSubmatch(m.Text, 10)
			if len(matchedDimensions) == 0 {
				bot.Send(m.Sender, "Send me correct dimensions, something like `128X128` or `256X256`", tb.ModeMarkdownV2)
				return
			}
			var dimensions []Dimensions
			for _, d := range matchedDimensions {
				values := strings.Split(d[0], "x")
				width, _ := strconv.ParseInt(values[0], 10, 32)
				height, _ := strconv.ParseInt(values[1], 10, 32)
				dimensions = append(dimensions, Dimensions{
					width:  int(width),
					height: int(height),
				})
			}
			// check if file is present in tg
			log.Info("Converting file " + imageFileId)
			file, err := bot.FileByID(imageFileId)
			if err != nil {
				log.WithField("fileId", imageFileId).Info("Can't access file, removing from queue")
				delete(lastChatImage, chatId)
			}
			// resize and reply
			fileContent, err := bot.GetFile(&file)
			if err != nil {
				log.Error(err)
				return
			}
			resized, err := resizeImage(fileContent, dimensions[0])
			if err != nil {
				log.Error(err)
				return
			}
			// todo leave and evict after some time
			delete(lastChatImage, chatId)
			a := &tb.Photo{File: tb.FromReader(bytes.NewBuffer(resized))}
			bot.Send(m.Sender, a)
		} else {
			bot.Send(m.Sender, "Send me an image first")
			return
		}
	})

	bot.Start()
}

func checkEnvVariable(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatal("Missing $" + name + " environment variable")
	}
	return value
}
