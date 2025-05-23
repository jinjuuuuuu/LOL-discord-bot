package discord

import (
	"context"
	"fmt"
	"log"
	"lol-assistant/gemini"
	"lol-assistant/league"
	"lol-assistant/notification"
	"lol-assistant/notifier"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// 전역 변수로 Gemini 클라이언트 선언
var geminiClient gemini.ChatSession

// Initialize는 봇 시작 시 호출되어 Gemini 클라이언트와 Riot API 키를 초기화합니다.
func Initialize() {
	geminiClient = gemini.NewGeminiClient()
	apiKey := os.Getenv("RIOT_GAMES_API_KEY")
	if apiKey == "" {
		log.Fatal("Error: RIOT_GAMES_API_KEY is not set")
	}
	log.Println("Bot initialized successfully")
}

// MessageHandler는 디스코드 메시지 이벤트 핸들러입니다.
func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 봇 자신의 메시지는 무시
	if m.Author.ID == s.State.User.ID {
		return
	}

	log.Printf("Received message: %s from user %s", m.Content, m.Author.Username)

	const maxLength = 2000

	// 임시 메시지를 전송하고 에러를 처리
	sendTempMessage := func(content string) (*discordgo.Message, error) {
		msg, err := s.ChannelMessageSend(m.ChannelID, content)
		if err != nil {
			log.Printf("임시 메시지 전송 오류: %v", err)
		}
		return msg, err
	}

	// 응답을 전송하며 Discord의 2000자 제한을 처리
	sendResponse := func(response string, tempMsg *discordgo.Message) error {
		if len(response) > maxLength {
			log.Printf("응답 길이 (%d)가 Discord 제한을 초과하여 분할 전송", len(response))
			for i := 0; i < len(response); i += maxLength {
				end := min(i+maxLength, len(response))
				if _, err := s.ChannelMessageSend(m.ChannelID, response[i:end]); err != nil {
					return fmt.Errorf("분할 응답 전송 실패: %v", err)
				}
			}
		} else {
			if _, err := s.ChannelMessageSend(m.ChannelID, response); err != nil {
				return fmt.Errorf("응답 전송 실패: %v", err)
			}
		}
		return s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
	}

	// "봇아"가 포함된 메시지 처리
	if strings.Contains(m.Content, "봇아") && !strings.HasPrefix(m.Content, "내가 마지막으로 플레이한 게임을 분석해줘") {
		tempMsg, err := sendTempMessage("답변을 생성하고 있어요")
		if err != nil {
			return
		}

		resp, err := geminiClient.ChatWithDiscord(context.Background(), m.Content)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("답변을 생성하지 못했어요: %v", err))
			log.Printf("Gemini API 오류: %v", err)
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
			return
		}

		if err := sendResponse(resp, tempMsg); err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("답변을 전송하지 못했어요: %v", err))
			log.Printf("응답 전송 오류: %v", err)
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
		}
		return
	}

	// 게임 분석 요청 처리
	if strings.HasPrefix(m.Content, "내가 마지막으로 플레이한 게임을 분석해줘") {
		tempMsg, err := sendTempMessage("게임 정보를 분석하고 있어요...")
		if err != nil {
			return
		}

		// 닉네임과 태그 파싱
		parts := strings.Split(m.Content, "|")
		if len(parts) != 2 {
			s.ChannelMessageSend(m.ChannelID, "잘못된 입력 형식입니다. 예: 내가 마지막으로 플레이한 게임을 분석해줘|닉네임#태그")
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
			return
		}

		nickTag := strings.Split(parts[1], "#")
		if len(nickTag) != 2 {
			s.ChannelMessageSend(m.ChannelID, "닉네임과 태그를 #으로 구분해주세요. 예: 닉네임#태그")
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
			return
		}

		nickName, tag := nickTag[0], nickTag[1]
		log.Printf("%s#%s의 매치 분석 처리 중", nickName, tag)

		// Riot API로 매치 데이터 가져오기
		_, summary, puuid, err := league.GetMatch(nickName, tag)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("게임 정보를 가져오지 못했어요: %v", err))
			log.Printf("Riot API 오류: %v", err)
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
			return
		}

		// Gemini 분석용 프롬프트 구성
		prompt := fmt.Sprintf(
			`소환사 %s#%s의 최근 게임 분석:
- PUUID: %s
- 게임 모드: %s
- 챔피언: %s
- 포지션: %s
- KDA: %s
- 승리 여부: %v
이 데이터를 바탕으로 챔피언 플레이 스타일, 강점, 약점, 게임 성과를 초보자도 이해할 수 있도록 자연스럽게 분석해 줘.`,
			nickName, tag, puuid, summary.GameMode, summary.ChampionName, summary.TeamPosition, summary.KDA, summary.Win)

		// Gemini API로 분석 요청
		resp, err := geminiClient.ChatWithDiscord(context.Background(), prompt)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("게임 분석을 생성하지 못했어요: %v", err))
			log.Printf("Gemini API 오류: %v", err)
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
			return
		}

		if err := sendResponse(resp, tempMsg); err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("분석 결과를 전송하지 못했어요: %v", err))
			log.Printf("분석 결과 전송 오류: %v", err)
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
		}
	}

	// 패치알림 켜기/끄기 처리
	switch strings.ToLower(m.Content) {
	case "!패치알림 켜기", "/패치알림 켜기":
		notification.EnablePatchNotification(m.Author.ID)
		s.ChannelMessageSend(m.ChannelID, "✅ 패치노트 알림을 켰어요!")
		return
	case "!패치알림 끄기", "/패치알림 끄기":
		notification.DisablePatchNotification(m.Author.ID)
		s.ChannelMessageSend(m.ChannelID, "❌ 패치노트 알림을 껐어요.")
		return
	}

	// 최근 패치노트 요청 명령어
	if strings.HasPrefix(m.Content, "!최근패치") || strings.Contains(m.Content, "최근 패치노트 알려줘") {
		tempMsg, err := sendTempMessage("최근 패치노트를 정리하고 있어요...")
		if err != nil {
			return
		}

		// 최신 패치노트 불러오기
		summary, champs, err := notifier.GetLatestPatchNoteSummary()
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("패치노트를 불러오지 못했어요: %v", err))
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
			return
		}

		// 챔피언 목록 문자열로 변환
		champList := strings.Join(champs, ", ")

		// Gemini 요약 요청 프롬프트 구성
		basePrompt := `안녕하세요, 소환사님! 다음은 리그 오브 레전드 25.10 패치노트의 상세 내용입니다:

%s

주요 변경 챔피언: %s

출력 형식은 다음과 같이 맞춰주세요:
### 25.10 패치 노트 요약
- **챔피언 변경**:
  - **챔피언 이름**: [요약]
- **아이템 변경** (만약 있다면): [요약]
- **시스템 변경** (만약 있다면): [요약]`
		prompt := fmt.Sprintf(basePrompt, summary, champList)

		resp, err := geminiClient.ChatWithDiscord(context.Background(), prompt)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("패치노트를 요약하지 못했어요: %v", err))
			log.Printf("Gemini API 오류: %v", err)
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
			return
		}

		if err := sendResponse(resp, tempMsg); err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("요약 전송에 실패했어요: %v", err))
			s.ChannelMessageDelete(m.ChannelID, tempMsg.ID)
		}
		return
	}
}

// min은 두 정수 중 작은 값을 반환합니다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}