// Command simulate runs a local browser chat UI that drives the real
// telegram.Bot.HandleUpdate code path — same command parsing, same
// authorization check, same service calls — through a fake Telegram client
// instead of the live Bot API. If a flow works here, it works identically
// against real Telegram, since only the transport (HTTP+JSON vs. this fake
// Sender) differs.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"ptas-bot/internal/billing"
	"ptas-bot/internal/db"
	"ptas-bot/internal/service"
	"ptas-bot/internal/telegram"
)

//go:embed static/index.html
var staticFiles embed.FS

// The simulator only ever has one user in one private chat, so these IDs are
// fixed rather than configurable.
const simOwnerID = 1
const simChatID = 1

func main() {
	dbPath := getEnv("DB_PATH", "data/ptas-sim.db")
	port := getEnv("PORT", "8090")
	waterRate := getEnvFloat("WATER_RATE_RIEL", 2500)
	elecRate := getEnvFloat("ELEC_RATE_RIEL", 1500)
	usdRate := getEnvFloat("USD_RATE_RIEL", 4000)

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer conn.Close()

	loc, err := time.LoadLocation("Asia/Phnom_Penh")
	if err != nil {
		loc = time.UTC
	}

	svc := service.New(conn, billing.Rates{
		WaterRiel: waterRate,
		ElecRiel:  elecRate,
		USDRiel:   usdRate,
	}, loc)

	sender := newFakeSender()
	bot := telegram.New(sender, svc, simOwnerID)

	html, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	router := gin.Default()
	router.GET("/", func(c *gin.Context) {
		c.FileFromFS("/", http.FS(html))
	})

	router.POST("/sim/message", func(c *gin.Context) {
		var req struct {
			Text string `json:"text"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		bot.HandleUpdate(buildMessageUpdate(simChatID, simOwnerID, req.Text))
		c.JSON(http.StatusOK, gin.H{"messages": sender.Drain()})
	})

	router.POST("/sim/callback", func(c *gin.Context) {
		var req struct {
			Data string `json:"data"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		bot.HandleUpdate(buildCallbackUpdate(simChatID, simOwnerID, req.Data))
		c.JSON(http.StatusOK, gin.H{"messages": sender.Drain()})
	})

	log.Printf("PTAS bot simulator: http://localhost:%s (data: %s)", port, dbPath)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}
