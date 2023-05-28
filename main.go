package main

import (
	"bytes"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

type Dimensions struct {
	width  int
	height int
}

type DocumentInfo struct {
	globalID     string
	originalName string
}

const (
	Greeting          = "Hello\\! I can resize images for you\\. Send me a file or an image\\."
	RequestDimensions = "Great\\! Now send me width and height, for example, `128x128`\\. You can also send multiple dimensions in one message\\: `64x64 128x128`\\."
	InvalidDimensions = "Send me correct dimensions, something like `128x128` or `256x256`\\."
	RequestImage      = "Send me an image first\\."
)

func main() {
	log.SetOutput(os.Stdout)

	// heroku healthcheck
	go http.ListenAndServe(":"+os.Getenv("PORT"), nil)

	// map of chat_id to last image in it
	var lastChatImage = make(map[int64]DocumentInfo)

	botToken := checkEnvVariable("BOT_TOKEN")

	bot, err := tb.NewBot(tb.Settings{
		Token:  botToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	bot.Handle(tb.OnDocument, func(c tb.Context) error {
		message := c.Message()
		chatID := c.Chat().ID

		// validate
		if message.Document.MIME != "image/png" && message.Document.MIME != "image/jpeg" {
			log.WithFields(log.Fields{
				"chatID":   strconv.FormatInt(chatID, 10),
				"fileId":   message.Document.FileID,
				"fileMime": message.Document.MIME,
			}).Info("Unsupported document format")
			return errors.New("unsupported document format")
		}

		documentInfo := DocumentInfo{
			globalID:     message.Document.FileID,
			originalName: message.Document.FileName,
		}
		lastChatImage[chatID] = documentInfo
		log.WithField("documentInfo", documentInfo).Info("New image queued for chat")

		// request dimensions
		return c.Send(RequestDimensions, tb.ModeMarkdownV2)
	})

	bot.Handle(tb.OnPhoto, func(c tb.Context) error {
		message := c.Message()
		chatID := c.Chat().ID
		documentInfo := DocumentInfo{
			globalID:     message.Photo.File.FileID,
			originalName: message.Photo.File.FileID,
		}
		lastChatImage[chatID] = documentInfo
		log.WithField("documentInfo", documentInfo).
			WithField("chatID", chatID).
			Info("New image queued for chat")

		// request dimensions
		return c.Send(RequestDimensions, tb.ModeMarkdownV2)
	})

	bot.Handle(tb.OnText, func(c tb.Context) error {
		message := c.Message()
		chatID := c.Chat().ID

		// greeting
		if message.Text == "/start" {
			return c.Send(Greeting, tb.ModeMarkdownV2)
		}
		if documentInfo, ok := lastChatImage[chatID]; ok {
			// parse dimensions
			dimensions := parseDimensions(message.Text)
			log.WithField("documentInfo", documentInfo).
				WithField("dimensions", dimensions).
				Info("Converting file for dimensions")
			if len(dimensions) == 0 {
				return c.Send(InvalidDimensions, tb.ModeMarkdownV2)
			}

			// check if file is present in tg
			file, err := bot.FileByID(documentInfo.globalID)
			if err != nil {
				log.WithField("documentInfo", documentInfo).
					Error("Can't access file, removing from queue")
				delete(lastChatImage, chatID)
			}

			// resize and reply
			fileType := mime.TypeByExtension(filepath.Ext(file.FilePath))
			for _, d := range dimensions {
				log.WithField("documentInfo", documentInfo).
					WithField("dimensions", d).
					Info("Converting file for ")
				fileContent, err := bot.File(&file)
				if err != nil {
					return err
				}
				resizedFile, err := resizeImage(fileContent, fileType, d)
				if err != nil {
					return err
				}
				resizedName := createNameForResizedFile(documentInfo.originalName, d, fileType)
				toSend := &tb.Document{
					File:      tb.FromReader(bytes.NewReader(resizedFile)),
					Thumbnail: nil,
					MIME:      fileType,
					FileName:  resizedName,
				}
				err = c.Send(toSend, tb.ModeMarkdownV2)
				if err != nil {
					return err
				}
			}
		} else {
			return c.Send(RequestImage, tb.ModeMarkdownV2)
		}
		return nil
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

func parseDimensions(message string) []Dimensions {
	var result []Dimensions
	r := regexp.MustCompile("[(\\d+)x(\\d+)]+")
	matchedDimensions := r.FindAllStringSubmatch(message, 10)
	if len(matchedDimensions) == 0 {
		return result
	}
	for _, d := range matchedDimensions {
		values := strings.Split(d[0], "x")
		width, _ := strconv.ParseInt(values[0], 10, 32)
		height, _ := strconv.ParseInt(values[1], 10, 32)
		result = append(result, Dimensions{
			width:  int(width),
			height: int(height),
		})
	}
	return result
}

func createNameForResizedFile(originalName string, dimensions Dimensions, fileType string) string {
	var nameWithoutExtensions string
	if pos := strings.LastIndexByte(originalName, '.'); pos != -1 {
		nameWithoutExtensions = originalName[:pos]
	} else {
		nameWithoutExtensions = originalName
	}
	var extension = filepath.Ext(originalName)
	if extension == "" {
		if fileType == "image/jpeg" {
			extension = ".jpeg"
		} else if fileType == "image/png" {
			extension = ".png"
		}
	}
	var sb strings.Builder
	sb.WriteString(nameWithoutExtensions)
	sb.WriteString("__")
	sb.WriteString(strconv.Itoa(dimensions.width))
	sb.WriteString("x")
	sb.WriteString(strconv.Itoa(dimensions.height))
	sb.WriteString(extension)
	return sb.String()
}
