package main

import (
	"Backend/internal/services"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	url := "https://job.mynavi.jp/27/pc/search/corp70553/outline.html"
	fmt.Println("=== マイナビ企業ページ取得テスト ===")
	fmt.Printf("URL: %s\n\n", url)

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SocAI/1.0)")
	req.Header.Set("Accept-Language", "ja,en-US;q=0.9,en;q=0.8")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("fetch error: %v", err)
	}
	defer resp.Body.Close()
	fmt.Printf("HTTP Status: %d\n\n", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("read error: %v", err)
	}

	data, err := services.ParseMynaviCompanyPagePublic(string(body))
	if err != nil {
		log.Fatalf("parse error: %v", err)
	}

	fmt.Println("=== パース結果 ===")
	fmt.Printf("企業名        : %s\n", data.Name)
	fmt.Printf("業種          : %s\n", data.Industry)
	fmt.Printf("従業員数      : %d名\n", data.EmployeeCount)
	fmt.Printf("設立年        : %d年\n", data.FoundedYear)
	fmt.Printf("所在地        : %s\n", data.Location)
	fmt.Printf("WebサイトURL  : %s\n", data.WebsiteURL)
	fmt.Printf("平均年齢      : %.1f歳\n", data.AverageAge)
	fmt.Printf("女性比率      : %.1f%%\n", data.FemaleRatio)
	fmt.Printf("事業内容      : %s\n", truncate(data.MainBusiness, 60))
	fmt.Printf("会社概要      : %s\n", truncate(data.Description, 60))
	fmt.Printf("福利厚生      : %s\n", truncate(data.WelfareDetails, 60))
	fmt.Printf("働き方        : %s\n", data.WorkStyle)
	fmt.Printf("企業文化      : %s\n", truncate(data.Culture, 60))
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "..."
	}
	return s
}
