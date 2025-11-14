package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	maxbot "github.com/max-messenger/max-bot-api-client-go"

	"proddy-bot/internal/handlers"
	"proddy-bot/internal/storage"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN environment variable is required")
	}

	api, _ := maxbot.New(botToken)
	storage := storage.NewMemoryStorage()
	handler := handlers.New(*storage)

	botCtx := context.Background()
	botInfo, err := api.Bots.GetBot(botCtx)
	if err != nil {
		log.Printf("Failed to get bot info: %v", err)
	} else {
		fmt.Printf("🤖 Bot: %s\n", botInfo.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		exit := make(chan os.Signal, 1)
		signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
		<-exit
		fmt.Println("\n🛑 Shutting down bot...")
		cancel()
	}()

	fmt.Println("🚀 Starting to process updates...")

	for update := range api.GetUpdates(ctx) {
		handler.HandleUpdate(ctx, api, update)
	}

	fmt.Println("👋 Bot stopped")
}
