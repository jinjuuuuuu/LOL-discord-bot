package notifier

import (
    "errors"
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"
    "unicode/utf8"

    "github.com/PuerkitoBio/goquery"
    "golang.org/x/text/encoding/korean"
    "golang.org/x/text/transform"
)

func GetLatestPatchNoteSummary() (string, []string, error) {
    client := &http.Client{Timeout: 10 * time.Second}
    url := "https://www.leagueoflegends.com/ko-kr/news/game-updates/"
    req, _ := http.NewRequest("GET", url, nil)
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("패치노트 페이지 요청 실패: %v", err)
        return "", nil, err
    }
    defer resp.Body.Close()

    log.Printf("패치노트 페이지 응답 상태: %d, Content-Type: %s", resp.StatusCode, resp.Header.Get("Content-Type"))
    if resp.StatusCode != http.StatusOK {
        return "", nil, fmt.Errorf("패치노트 페이지 요청 실패, 상태 코드: %d", resp.StatusCode)
    }

    // EUC-KR을 UTF-8로 변환 (페이지가 EUC-KR일 가능성 대비)
    utf8Reader := transform.NewReader(resp.Body, korean.EUCKR.NewDecoder())
    doc, err := goquery.NewDocumentFromReader(utf8Reader)
    if err != nil {
        log.Printf("HTML 파싱 실패: %v", err)
        return "", nil, err
    }

    // 메타 태그 디버깅
    doc.Find("meta").Each(func(i int, s *goquery.Selection) {
        if charset, exists := s.Attr("charset"); exists {
            log.Printf("메타 태그 charset: %s", charset)
        }
        if httpEquiv, exists := s.Attr("http-equiv"); exists && httpEquiv == "Content-Type" {
            if content, exists := s.Attr("content"); exists {
                log.Printf("메타 태그 Content-Type: %s", content)
            }
        }
    })

    var patchURL, patchVersion string
    doc.Find("a[data-testid='articlefeaturedcard-component']").EachWithBreak(func(i int, s *goquery.Selection) bool {
        href, exists := s.Attr("href")
        if exists && strings.Contains(href, "/ko-kr/news/game-updates/patch-") {
            patchURL = "https://www.leagueoflegends.com" + href
            patchVersion = strings.TrimSpace(s.Find("div[data-testid='card-title']").Text())
            return false
        }
        return true
    })

    if patchURL == "" {
        log.Printf("최신 패치노트 링크를 찾을 수 없습니다. 페이지: %s", url)
        return "", nil, errors.New("최신 패치노트 링크를 찾을 수 없습니다")
    }
    log.Printf("최신 패치노트 링크: %s, 버전: %s", patchURL, patchVersion)

    req, _ = http.NewRequest("GET", patchURL, nil)
    resp, err = client.Do(req)
    if err != nil {
        log.Printf("패치노트 상세 페이지 요청 실패: %v", err)
        return "", nil, err
    }
    defer resp.Body.Close()

    log.Printf("패치노트 상세 페이지 응답 상태: %d, Content-Type: %s", resp.StatusCode, resp.Header.Get("Content-Type"))
    if resp.StatusCode != http.StatusOK {
        return "", nil, fmt.Errorf("패치노트 상세 페이지 요청 실패, 상태 코드: %d", resp.StatusCode)
    }

    utf8Reader = transform.NewReader(resp.Body, korean.EUCKR.NewDecoder())
    doc, err = goquery.NewDocumentFromReader(utf8Reader)
    if err != nil {
        log.Printf("상세 페이지 HTML 파싱 실패: %v", err)
        return "", nil, err
    }

    if patchVersion == "" {
        doc.Find("h1, div[data-testid='card-title']").EachWithBreak(func(i int, s *goquery.Selection) bool {
            if strings.Contains(s.Text(), "패치") {
                patchVersion = strings.TrimSpace(s.Text())
                return false
            }
            return true
        })
    }

    if patchVersion == "" {
        log.Printf("패치 버전을 찾을 수 없습니다. 페이지: %s", patchURL)
        return "", nil, errors.New("패치 버전을 찾을 수 없습니다")
    }

    var updatedChamps []string
    var summaryBuilder strings.Builder
    summaryBuilder.WriteString(fmt.Sprintf("### %s 요약\n", patchVersion))

    // 챔피언 변경 내용 파싱
    doc.Find("h3.change-title").Each(func(i int, s *goquery.Selection) {
        champName := cleanText(strings.TrimSpace(s.Text()))
        if champName == "" {
            champName = cleanText(strings.TrimSpace(s.Find("a").Text())) // <a> 태그에서 챔피언 이름 추출
        }
        if champName != "" && !contains(updatedChamps, champName) {
            updatedChamps = append(updatedChamps, champName)
            summaryBuilder.WriteString(fmt.Sprintf("- **%s**:\n", champName))
            summaryText := cleanText(strings.TrimSpace(s.Next().Find("p.summary").Text())) // <p class="summary">에서 요약 추출
            if summaryText != "" {
                summaryBuilder.WriteString(fmt.Sprintf("  - 요약: %s\n", truncate(summaryText, 100)))
                log.Printf("챔피언: %s, 요약: %s", champName, summaryText)
            }
        }
    })

    // 아이템 변경 내용 파싱 (기존 로직 유지)
    doc.Find("h3.change-detail-title.ability-title").Each(func(i int, s *goquery.Selection) {
        itemName := cleanText(strings.TrimSpace(s.Text()))
        if itemName != "" {
            summaryBuilder.WriteString(fmt.Sprintf("- **%s**:\n", itemName))
            ul := s.Parent().Find("ul").First()
            ul.Find("li").Each(func(j int, li *goquery.Selection) {
                text := cleanText(strings.TrimSpace(li.Text()))
                if text != "" {
                    summaryBuilder.WriteString(fmt.Sprintf("  - %s\n", text))
                    log.Printf("아이템: %s, 변경 내용: %s", itemName, text)
                }
            })
        }
    })

    // 룬 변경 내용 파싱 (기존 로직 유지)
    doc.Find("div.patch-change-block").Each(func(i int, s *goquery.Selection) {
        runeTitle := cleanText(strings.TrimSpace(s.Find("h3.change-title").Text()))
        if runeTitle == "" {
            runeTitle = cleanText(strings.TrimSpace(s.Find("h3.change-title a").Text()))
        }
        if runeTitle == "" {
            return
        }
        summaryBuilder.WriteString(fmt.Sprintf("- **%s (룬 변경)**:\n", runeTitle))
        s.Find("ul li").Each(func(j int, li *goquery.Selection) {
            changeText := cleanText(strings.TrimSpace(li.Text()))
            if changeText != "" {
                summaryBuilder.WriteString(fmt.Sprintf("  - %s\n", changeText))
                log.Printf("룬: %s, 변경 내용: %s", runeTitle, changeText)
            }
        })
    })

    summary := summaryBuilder.String()
    if summary == fmt.Sprintf("### %s 요약\n", patchVersion) {
        log.Printf("패치노트 내용 파싱 실패. 페이지: %s", patchURL)
        return fmt.Sprintf("최신 패치노트(%s)를 불러오지 못했어요. 공식 웹사이트(%s)를 확인해 보세요.", patchVersion, patchURL), nil, nil
    }

    return summary, updatedChamps, nil
}

func cleanText(text string) string {
    if utf8.ValidString(text) {
        return text
    }
    cleaned, _, err := transform.String(korean.EUCKR.NewDecoder(), text)
    if err != nil {
        log.Printf("텍스트 변환 실패: %v", err)
        return ""
    }
    return cleaned
}

func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}