package league

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// MatchSummary는 게임 요약 정보를 담는 구조체
type MatchSummary struct {
	ChampionName string
	KDA          string
	Win          bool
	TeamPosition string
	GameMode     string
}

// GetMatch는 닉네임과 태그를 받아 최근 매치 정보를 조회하고 요약 정보를 반환
func GetMatch(name, tag string) (map[string]interface{}, *MatchSummary, string, error) {
	log.Println("Starting GetMatch for", name+"#"+tag)

	apiKey := os.Getenv("RIOT_GAMES_API_KEY")
	if apiKey == "" {
		log.Println("Error: RIOT_GAMES_API_KEY is not set")
		return nil, nil, "", fmt.Errorf("RIOT_GAMES_API_KEY is not set")
	}
	log.Println("API Key loaded successfully")

	// 1. 닉네임 + 태그로 PUUID 조회
	accountURL := fmt.Sprintf("https://asia.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s?api_key=%s", name, tag, apiKey)
	log.Printf("Requesting account info: %s", accountURL)
	resp, err := http.Get(accountURL)
	if err != nil {
		log.Printf("Error fetching account: %v", err)
		return nil, nil, "", fmt.Errorf("failed to fetch account: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing account response body: %v", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading account response: %v", err)
		return nil, nil, "", fmt.Errorf("failed to read account response: %v", err)
	}

	log.Printf("Account API response (status %d): %s", resp.StatusCode, string(respBody))
	if resp.StatusCode != 200 {
		log.Printf("Account API error: %d, response: %s", resp.StatusCode, string(respBody))
		return nil, nil, "", fmt.Errorf("account API error: %d, %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err = json.Unmarshal(respBody, &result); err != nil {
		log.Printf("Error parsing account response: %v", err)
		return nil, nil, "", fmt.Errorf("failed to parse account response: %v", err)
	}

	puuid, ok := result["puuid"].(string)
	if !ok || puuid == "" {
		log.Println("Error: puuid not found in account response")
		return nil, nil, "", fmt.Errorf("puuid not found in account response")
	}
	log.Printf("PUUID retrieved: %s", puuid)

	// 2. PUUID로 최근 매치 ID 조회
	matchListURL := fmt.Sprintf("https://asia.api.riotgames.com/lol/match/v5/matches/by-puuid/%s/ids?start=0&count=1&api_key=%s", puuid, apiKey)
	log.Printf("Requesting match list: %s", matchListURL)
	resp, err = http.Get(matchListURL)
	if err != nil {
		log.Printf("Error fetching match list: %v", err)
		return nil, nil, "", fmt.Errorf("failed to fetch match list: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing match list response body: %v", err)
		}
	}()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading match list response: %v", err)
		return nil, nil, "", fmt.Errorf("failed to read match list response: %v", err)
	}

	log.Printf("Match List API response (status %d): %s", resp.StatusCode, string(respBody))
	if resp.StatusCode != 200 {
		log.Printf("Match list API error: %d, response: %s", resp.StatusCode, string(respBody))
		return nil, nil, "", fmt.Errorf("match list API error: %d, %s", resp.StatusCode, string(respBody))
	}

	var matchIds []string
	if err = json.Unmarshal(respBody, &matchIds); err != nil {
		log.Printf("Error parsing match list response: %v", err)
		return nil, nil, "", fmt.Errorf("failed to parse match list response: %v", err)
	}

	if len(matchIds) == 0 {
		log.Printf("No matches found for puuid: %s", puuid)
		return nil, nil, "", fmt.Errorf("no matches found for puuid: %s", puuid)
	}
	log.Printf("Match ID retrieved: %s", matchIds[0])

	// 3. 매치 ID로 게임 상세 정보 조회
	matchDetailURL := fmt.Sprintf("https://asia.api.riotgames.com/lol/match/v5/matches/%s?api_key=%s", matchIds[0], apiKey)
	log.Printf("Requesting match details: %s", matchDetailURL)
	resp, err = http.Get(matchDetailURL)
	if err != nil {
		log.Printf("Error fetching match details: %v", err)
		return nil, nil, "", fmt.Errorf("failed to fetch match details: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing match details response body: %v", err)
		}
	}()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading match details response: %v", err)
		return nil, nil, "", fmt.Errorf("failed to read match details response: %v", err)
	}

	if resp.StatusCode != 200 {
		log.Printf("Match details API error: %d, response: %s", resp.StatusCode, string(respBody))
		return nil, nil, "", fmt.Errorf("match details API error: %d, %s", resp.StatusCode, string(respBody))
	}

	var match map[string]interface{}
	if err = json.Unmarshal(respBody, &match); err != nil {
		log.Printf("Error parsing match details response: %v", err)
		return nil, nil, "", fmt.Errorf("failed to parse match details response: %v", err)
	}

	// 4. 게임 정보 요약
	summary, err := extractMatchSummary(match, puuid)
	if err != nil {
		log.Printf("Error extracting match summary: %v", err)
		return match, nil, puuid, fmt.Errorf("failed to extract match summary: %v", err)
	}

	log.Printf("Match summary extracted: Champion=%s, KDA=%s, Win=%v, Position=%s, GameMode=%s",
		summary.ChampionName, summary.KDA, summary.Win, summary.TeamPosition, summary.GameMode)

	return match, summary, puuid, nil
}

// extractMatchSummary는 match 데이터에서 필요한 정보를 추출하여 요약
func extractMatchSummary(match map[string]interface{}, puuid string) (*MatchSummary, error) {
	info, ok := match["info"].(map[string]interface{})
	if !ok {
		log.Println("Error: info field not found in match data")
		return nil, fmt.Errorf("info field not found in match data")
	}

	gameMode, _ := info["gameMode"].(string)

	participants, ok := info["participants"].([]interface{})
	if !ok {
		log.Println("Error: participants field not found in match data")
		return nil, fmt.Errorf("participants field not found in match data")
	}

	for _, p := range participants {
		participant, ok := p.(map[string]interface{})
		if !ok {
			log.Println("Error: invalid participant data")
			continue
		}
		participantPuuid, ok := participant["puuid"].(string)
		if !ok {
			log.Println("Error: puuid not found in participant data")
			continue
		}
		if participantPuuid == puuid {
			championName, _ := participant["championName"].(string)
			kills, _ := participant["kills"].(float64)
			deaths, _ := participant["deaths"].(float64)
			assists, _ := participant["assists"].(float64)
			win, _ := participant["win"].(bool)
			teamPosition, _ := participant["teamPosition"].(string)

			kda := fmt.Sprintf("%d/%d/%d", int(kills), int(deaths), int(assists))
			return &MatchSummary{
				ChampionName: championName,
				KDA:          kda,
				Win:          win,
				TeamPosition: teamPosition,
				GameMode:     gameMode,
			}, nil
		}
	}

	log.Printf("Error: participant not found for puuid: %s", puuid)
	return nil, fmt.Errorf("participant not found for puuid: %s", puuid)
}