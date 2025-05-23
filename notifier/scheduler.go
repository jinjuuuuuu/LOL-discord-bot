package notifier

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
)

// 외부에서 설정할 수 있도록 변수 추가
var DiscordSession *discordgo.Session
var NotifyChannelID string // 알림을 보낼 디스코드 채널 ID

func StartScheduler() {
	c := cron.New()

	// 매 시간마다 실행
	_, err := c.AddFunc("@every 1h", func() {
		log.Println("⏰ 패치노트 확인 작업 실행 중...")

		// Riot API에서 패치 요약 및 자주 쓰는 챔피언 관련 내용 가져오기
		summary, updatedChamps, err := GetLatestPatchNoteSummary()
		if err != nil {
			log.Printf("❌ 패치노트 확인 실패: %v", err)
			return
		}

		// 알림 전송
		CheckAndNotifyPatchNotes(DiscordSession, NotifyChannelID, summary, updatedChamps)
	})

	if err != nil {
		log.Fatalf("스케줄러 등록 실패: %v", err)
	}

	c.Start()
	log.Println("✅ 패치노트 스케줄러 시작됨")
}
