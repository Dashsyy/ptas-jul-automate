package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"ptas-bot/internal/billing"
	"ptas-bot/internal/config"
	"ptas-bot/internal/db"
	"ptas-bot/internal/service"
	"ptas-bot/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	loc, err := time.LoadLocation("Asia/Phnom_Penh")
	if err != nil {
		loc = time.UTC
	}

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer conn.Close()

	svc := service.New(conn, billing.Rates{
		WaterRiel: cfg.WaterRateRiel,
		ElecRiel:  cfg.ElecRateRiel,
		USDRiel:   cfg.USDRateRiel,
	}, loc)

	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	bot := telegram.New(api, svc, cfg.OwnerID, api.Self.UserName)
	log.Printf("authorized as @%s", api.Self.UserName)
	if err := bot.RegisterCommands(); err != nil {
		log.Printf("register commands: %v", err)
	}
	if err := telegram.SetCommandsMenuButton(api); err != nil {
		log.Printf("set menu button: %v", err)
	}

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	if cfg.PublicBaseURL != "" {
		runWebhook(router, api, bot, cfg)
	} else {
		go runLongPolling(api, bot)
		if err := router.Run(":" + cfg.Port); err != nil {
			log.Fatalf("http server: %v", err)
		}
	}
}

func runWebhook(router *gin.Engine, api *tgbotapi.BotAPI, bot *telegram.Bot, cfg config.Config) {
	path := "/webhook/" + cfg.WebhookSecret
	webhookURL := cfg.PublicBaseURL + path

	wh, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		log.Fatalf("build webhook: %v", err)
	}
	if _, err := api.Request(wh); err != nil {
		log.Fatalf("set webhook: %v", err)
	}
	log.Printf("webhook set to %s", webhookURL)

	router.POST(path, func(c *gin.Context) {
		var update tgbotapi.Update
		if err := json.NewDecoder(c.Request.Body).Decode(&update); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		bot.HandleUpdate(update)
		c.Status(http.StatusOK)
	})

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func runLongPolling(api *tgbotapi.BotAPI, bot *telegram.Bot) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := api.GetUpdatesChan(u)
	log.Println("long-polling for updates")
	for update := range updates {
		bot.HandleUpdate(update)
	}
}
