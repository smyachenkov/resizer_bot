package main

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Dimensions struct {
	width  int
	height int
}

type DocumentInfo struct {
	globalId     string
	originalName string
}

const (
	Greeting          = "Hello\\! I can resize images for you. Send me a file or an image\\."
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

	bot.Handle(tb.OnDocument, func(m *tb.Message) {
		chatId := m.Chat.ID
		// validate
		if m.Document.MIME != "image/png" && m.Document.MIME != "image/jpeg" {
			log.WithFields(log.Fields{
				"chatId":   strconv.FormatInt(chatId, 10),
				"fileId":   m.Document.FileID,
				"fileMime": m.Document.MIME,
			}).Info("Unsupported document format")
			return
		}

		documentInfo := DocumentInfo{
			globalId:     m.Document.FileID,
			originalName: m.Document.FileName,
		}
		lastChatImage[chatId] = documentInfo
		log.WithField("documentInfo", documentInfo).Info("New image queued for chat")

		// request dimensions
		bot.Send(m.Sender, RequestDimensions, tb.ModeMarkdownV2)
	})

	bot.Handle(tb.OnPhoto, func(m *tb.Message) {
		chatId := m.Chat.ID
		documentInfo := DocumentInfo{
			globalId:     m.Photo.File.FileID,
			originalName: m.Photo.File.FileID,
		}
		lastChatImage[chatId] = documentInfo
		log.WithField("documentInfo", documentInfo).Info("New image queued for chat")

		// request dimensions
		bot.Send(m.Sender, RequestDimensions, tb.ModeMarkdownV2)
	})

	bot.Handle(tb.OnText, func(m *tb.Message) {
		// greeting
		if m.Text == "/start" {
			bot.Send(m.Sender, Greeting, tb.ModeMarkdownV2)
			return
		}
		chatId := m.Chat.ID
		if documentInfo, ok := lastChatImage[chatId]; ok {
			// parse dimensions
			dimensionsToResize := parseDimensions(m.Text)
			if len(dimensionsToResize) == 0 {
				bot.Send(m.Sender, InvalidDimensions, tb.ModeMarkdownV2)
				return
			}

			// check if file is present in tg
			log.WithField("documentInfo", documentInfo).Info("Converting file")
			file, err := bot.FileByID(documentInfo.globalId)
			if err != nil {
				log.WithField("documentInfo", documentInfo).Info("Can't access file, removing from queue")
				delete(lastChatImage, chatId)
			}

			// resize and reply
			fileType := mime.TypeByExtension(filepath.Ext(file.FilePath))
			for _, dimensions := range dimensionsToResize {
				fileContent, err := bot.GetFile(&file)
				if err != nil {
					log.Error(err)
					return
				}
				resizedFile, err := resizeImage(fileContent, fileType, dimensions)
				if err != nil {
					log.Error(err)
				}
				resizedName := createNameForResizedFile(documentInfo.originalName, dimensions, fileType)
				toSend := &tb.Document{
					File:      tb.FromReader(bytes.NewReader(resizedFile)),
					Thumbnail: nil,
					MIME:      fileType,
					FileName:  resizedName,
				}
				bot.Send(m.Sender, toSend)
			}
		} else {
			bot.Send(m.Sender, RequestImage, tb.ModeMarkdownV2)
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
