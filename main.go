package main

import (
	"fmt"
	"log"
	"lol-assistant/discord"
	"lol-assistant/notifier"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	// .env 파일에서 환경 변수 로드
	if err := godotenv.Load("./.env"); err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	// Gemini 클라이언트 초기화
	discord.Initialize()

	// 디스코드 봇 토큰 설정
	token := fmt.Sprintf("Bot %s", os.Getenv("BOT_TOKEN"))
	dg, err := discordgo.New(token)
	if err != nil {
		log.Fatal("Error creating Discord session:", err)
	}

	// 메시지 핸들러 등록
	dg.AddHandler(discord.MessageHandler)
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// 디스코드 연결 시작
	if err := dg.Open(); err != nil {
		log.Fatal("Error opening connection:", err)
	}

	fmt.Println("Bot is now running.")

	// 알림에 필요한 정보 전달
	notifier.DiscordSession = dg
	notifier.NotifyChannelID = os.Getenv("PATCH_NOTIFY_CHANNEL_ID")

	// 스케줄러 고루틴으로 실행
	go notifier.StartScheduler()

	// 종료 시그널 핸들링 고루틴
	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

		sig := <-sc
		fmt.Println("\nReceived shutdown signal:", sig)
		fmt.Println("Shutting down bot...")

		if err := dg.Close(); err != nil {
			log.Fatal("Failed to close Discord session:", err)
		}
		os.Exit(0)
	}()

	// 메인 고루틴 블로킹
	select {}
}