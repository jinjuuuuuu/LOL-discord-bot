package notifier

import (
	"context"
	"log"
	"lol-assistant/gemini"
	"lol-assistant/notification"

	"github.com/bwmarrin/discordgo"
)

var prevPatchVersion string

func CheckAndNotifyPatchNotes(s *discordgo.Session, channelID string, patchSummary string, updatedChamps []string) {
	// 이미 전송한 버전이면 무시
	if patchSummary == prevPatchVersion {
		return
	}
	prevPatchVersion = patchSummary

	log.Println("패치노트 업데이트 감지됨. 알림 전송 중...")

	client := gemini.NewGeminiClient()

	for _, userID := range notification.GetAllNotifiedUsers() {
		prompt := "다음은 리그 오브 레전드의 최신 패치노트 요약입니다.\n"
		if len(updatedChamps) > 0 {
			prompt += "🎯 당신이 자주 사용하는 챔피언 관련 주요 변경:\n"
			for _, champ := range updatedChamps {
				prompt += "- " + champ + "\n"
			}
		}
		prompt += "\n🧠 전체 패치 요약을 포함해서 사용자에게 설명해줘: " + patchSummary

		resp, err := client.ChatWithDiscord(context.Background(), prompt)
		if err != nil {
			log.Printf("패치노트 요약 생성 실패: %v", err)
			continue
		}

		// DM 전송
		_, err = s.ChannelMessageSend(channelID, "<@"+userID+">님!\n"+resp)
		if err != nil {
			log.Printf("DM 전송 실패: %v", err)
		}
	}
}
