package discord

import (
	"context"
	"fmt"
	"log"
	"mymodule/gemini"
	"mymodule/league"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// 전역 변수로 Gemini 클라이언트 선언
var geminiClient gemini.ChatSession

// Initialize 함수는 봇 시작 시 한 번만 호출되어
// Gemini AI 클라이언트를 초기화합니다.
func Initialize() {
	geminiClient = gemini.NewGeminiClient()
}

func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Content == "ping" {
		s.ChannelMessageSend(m.ChannelID, "pong")
	}

	if m.Content == "여~" {
		s.ChannelMessageSend(m.ChannelID, "오셨습니까 행님!")
	}

	if strings.Contains(m.Content, "봇아") {
		delMsg, err := s.ChannelMessageSend(m.ChannelID, "답변 생성중...")
		if err != nil {
			log.Println("답변 생성 실패", err)
			return
		}

		resp, err := geminiClient.ChatWithDiscord(context.Background(), m.Content)
		if err != nil {
			// 응답 생성 실패 시 에러 메시지 전송
			_, sendErr := s.ChannelMessageSend(m.ChannelID, "답변을 생성하지 못했어요")
			if sendErr != nil {
				log.Fatalln("답변생성 실패", err)
				return
			}
			log.Println("gemini api 에러", err)
			return
		}

		msg, err := s.ChannelMessageSend(m.ChannelID, resp)
		if err != nil {
			// 응답 전송 실패 시 에러 메시지 전송
			_, sendErr := s.ChannelMessageSend(m.ChannelID, "답변을 생성하지 못했어요")
			if sendErr != nil {
				log.Fatalln("답변생성 실패", err)
				return
			}
			log.Println("gemini api 에러", err)
			return
		} else {
			log.Println(msg)
		}

		err = s.ChannelMessageDelete(m.ChannelID, delMsg.ID)
		if err != nil {
			log.Println("메세지 삭제 실패", err)
			return
		}
	} else if strings.HasPrefix(m.Content, "내가 마지막으로 플레이한 게임을 분석해줘") {
		// ex: "내가 마지막으로 플레이한 게임을 분석해줘|닉네임#태그
		delMsg, err := s.ChannelMessageSend(m.ChannelID, "답변을 생성하고 있어요")
		if err != nil {
			log.Println("답변을 생성하고 있어요 실패", err)
			return
		}

		split := strings.Split(m.Content, "|")
		nickName := strings.Split(split[1], "#")[0]
		tag := strings.Split(split[1], "#")[1]

		gameInfo, puuid, err := league.GetMatch(nickName, tag)
		if err != nil {
			if err != nil {
				// 응답 전송 실패 시 에러 메시지 전송
				_, sendErr := s.ChannelMessageSend(m.ChannelID, "답변을 생성하지 못했어요")
				if sendErr != nil {
					log.Fatalln("답변생성 실패", err)
					return
				}
				log.Println("gemini api 에러", err)
				return
			}
			err = s.ChannelMessageDelete(m.ChannelID, delMsg.ID)
			if err != nil {
				log.Println("메세지 삭제 실패", err)
				return
			}
		}
		matchReq := fmt.Sprintf("%s |  puuid: %s, 게임정보: %s | 나의 puuid와 닉네임, 게임태그를 사용해서 내가 플레이한 캐릭터의 정보를 분석해줘", m.Content, puuid, gameInfo)

		resp, err := geminiClient.ChatWithDiscord(context.Background(), matchReq)
		if err != nil {
			// 응답 생성 실패 시 에러 메시지 전송
			_, sendErr := s.ChannelMessageSend(m.ChannelID, "답변을 생성하지 못했어요")
			if sendErr != nil {
				log.Fatalln("답변생성 실패", err)
				return
			}
			log.Println("gemini api 에러", err)
			return
		}
		log.Println(resp)

		_, err = s.ChannelMessageSend(m.ChannelID, resp)
		if err != nil {
			_, sendErr := s.ChannelMessageSend(m.ChannelID, "답변을 생성하지 못했어요")
			if sendErr != nil {
				log.Fatalln("답변생성 실패", err)
				return
			}
			log.Println("gemini api 에러", err)
			return
		}
		err = s.ChannelMessageDelete(m.ChannelID, delMsg.ID)
		if err != nil {
			log.Println("메세지 삭제 실패", err)
			return
		}
	}
}
