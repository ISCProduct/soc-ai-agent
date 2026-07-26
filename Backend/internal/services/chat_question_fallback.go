package services

import (
	"strings"
)

func (s *ChatService) fallbackQuestionForCategory(category string, jobCategoryID uint, targetLevel string) string {
	switch category {
	case "技術志向":
		return s.techInterestQuestion(jobCategoryID, targetLevel)
	case "コミュニケーション能力":
		if targetLevel == "中途" {
			return "業務で関係者と調整した経験はありますか？どんな場面で、どのように進めましたか？"
		}
		return "グループワークであなたがよく担当する役割は何ですか？（例: アイデア出し、まとめ役、サポートなど）"
	case "リーダーシップ":
		if targetLevel == "中途" {
			return "業務でチームや案件をリードした経験はありますか？どのように進めましたか？"
		}
		return "グループで何かをまとめた経験はありますか？どんな場面でしたか？"
	case "チームワーク":
		if targetLevel == "中途" {
			return "チームで協力して成果を出した経験はありますか？あなたの役割も教えてください。"
		}
		return "サークルや授業で、チームで取り組んだ経験はありますか？どんな役割でしたか？"
	case "問題解決力":
		if targetLevel == "中途" {
			return "業務で課題が起きたとき、どのように解決しましたか？最近の例を教えてください。"
		}
		return "課題やレポートで困ったとき、どのように解決しましたか？最近の例を教えてください。"
	case "創造性・発想力":
		if targetLevel == "中途" {
			return "業務で改善や工夫を提案した経験はありますか？どんな内容でしたか？"
		}
		return "新しいアイデアを出した経験はありますか？どんな工夫をしましたか？"
	case "計画性・実行力":
		if targetLevel == "中途" {
			return "業務で計画を立てて実行した経験を教えてください。どのように進めましたか？"
		}
		return "何かを計画して実行した経験を教えてください。どのように進めましたか？"
	case "学習意欲・成長志向":
		if targetLevel == "中途" {
			return "業務に役立てるために学んだことはありますか？直近の例があれば教えてください。"
		}
		return "新しいことを学ぶとき、どうやって学習を進めますか？直近で学んだことはありますか？"
	case "ストレス耐性・粘り強さ":
		if targetLevel == "中途" {
			return "業務で困難に直面したとき、どのように乗り越えましたか？具体例があれば教えてください。"
		}
		return "困難に直面したとき、どのように乗り越えましたか？具体例があれば教えてください。"
	case "ビジネス思考・目標志向":
		if targetLevel == "中途" {
			return "業務で目標を立てて達成した経験はありますか？どんな目標でしたか？"
		}
		return "目標を立てて達成した経験はありますか？どんな目標でしたか？"
	default:
		return ""
	}
}

func (s *ChatService) fallbackQuestionsForCategory(category string, jobCategoryID uint, targetLevel string) []string {
	switch category {
	case "技術志向":
		return []string{
			s.techInterestQuestion(jobCategoryID, targetLevel),
			"最近触れた技術やツールはありますか？どんなことでも大丈夫です。",
		}
	case "コミュニケーション能力":
		if targetLevel == "中途" {
			return []string{
				"業務で相手に説明するとき、意識していることは何ですか？",
				"関係者とのやり取りで工夫したことはありますか？",
			}
		}
		return []string{
			"人に説明するとき、意識していることは何ですか？",
			"授業やサークルで発表した経験はありますか？",
		}
	case "リーダーシップ":
		if targetLevel == "中途" {
			return []string{
				"業務で主導したことはありますか？どんな場面でしたか？",
				"周りを巻き込んで進めた経験はありますか？",
			}
		}
		return []string{
			"自分から提案したりまとめ役をしたことはありますか？",
			"人をまとめた経験があれば教えてください。",
		}
	case "チームワーク":
		if targetLevel == "中途" {
			return []string{
				"チームで協力して進めた仕事はありますか？",
				"メンバーと連携する際に意識していることは？",
			}
		}
		return []string{
			"グループで協力した経験はありますか？",
			"チームで取り組んだときの役割を教えてください。",
		}
	case "問題解決力":
		if targetLevel == "中途" {
			return []string{
				"業務で困ったとき、どう解決しましたか？",
				"トラブル対応で工夫したことはありますか？",
			}
		}
		return []string{
			"困ったとき、どうやって解決しましたか？",
			"課題で行き詰まったときの対処法を教えてください。",
		}
	case "創造性・発想力":
		if targetLevel == "中途" {
			return []string{
				"業務で改善案を出したことはありますか？",
				"新しいアイデアを提案した経験はありますか？",
			}
		}
		return []string{
			"新しいアイデアを出した経験はありますか？",
			"いつもと違う工夫をしたことはありますか？",
		}
	case "計画性・実行力":
		if targetLevel == "中途" {
			return []string{
				"業務で計画を立てて進めた経験はありますか？",
				"期限に向けて進めた仕事はありますか？",
			}
		}
		return []string{
			"計画を立てて進めた経験はありますか？",
			"期限を意識して進めたことはありますか？",
		}
	case "学習意欲・成長志向":
		if targetLevel == "中途" {
			return []string{
				"最近学んだことはありますか？",
				"仕事のために学習したことはありますか？",
			}
		}
		return []string{
			"最近学んだことはありますか？",
			"新しく始めたことはありますか？",
		}
	case "ストレス耐性・粘り強さ":
		if targetLevel == "中途" {
			return []string{
				"大変だった仕事をどう乗り越えましたか？",
				"プレッシャーのある場面での対処を教えてください。",
			}
		}
		return []string{
			"大変なとき、どうやって乗り越えましたか？",
			"うまくいかない時の気持ちの切り替え方は？",
		}
	case "ビジネス思考・目標志向":
		if targetLevel == "中途" {
			return []string{
				"目標を立てて取り組んだ経験はありますか？",
				"成果を意識して進めた仕事はありますか？",
			}
		}
		return []string{
			"目標を立てて取り組んだ経験はありますか？",
			"目標達成のために工夫したことはありますか？",
		}
	default:
		return []string{s.fallbackQuestionForCategory(category, jobCategoryID, targetLevel)}
	}
}

func (s *ChatService) selectFallbackQuestion(category string, jobCategoryID uint, targetLevel string, askedTexts map[string]bool) string {
	options := s.fallbackQuestionsForCategory(category, jobCategoryID, targetLevel)
	for _, q := range options {
		if strings.TrimSpace(q) == "" {
			continue
		}
		if !askedTexts[q] {
			return q
		}
	}
	generic := []string{}
	if targetLevel == "中途" {
		generic = []string{
			"最近取り組んだ仕事やタスクはありますか？簡単に教えてください。",
			"仕事で工夫したことがあれば教えてください。",
		}
	} else {
		generic = []string{
			"最近頑張ったことはありますか？",
			"新しく挑戦したことはありますか？",
		}
	}
	for _, q := range generic {
		if strings.TrimSpace(q) == "" {
			continue
		}
		if !askedTexts[q] {
			return q
		}
	}
	return ""
}

func (s *ChatService) techInterestQuestion(jobCategoryID uint, targetLevel string) string {
	code := s.getJobCategoryCode(jobCategoryID)
	if targetLevel == "中途" {
		switch {
		case strings.HasPrefix(code, "ENG"):
			return "業務で使った技術や、最近取り組んだ開発について教えてください。"
		case strings.HasPrefix(code, "SALES"):
			return "営業活動でITツールや仕組みを活用した経験はありますか？どのように使いましたか？"
		case strings.HasPrefix(code, "MKT"):
			return "データやデジタルを使った施策の経験はありますか？内容を教えてください。"
		case strings.HasPrefix(code, "HR"):
			return "人事領域でITツールや仕組みを使った経験はありますか？具体例があれば教えてください。"
		case strings.HasPrefix(code, "FIN"):
			return "数値管理や分析で使ったツール・仕組みがあれば教えてください。"
		case strings.HasPrefix(code, "CONS"):
			return "業務でデータやツールを使って課題整理をした経験はありますか？"
		default:
			return "業務でITツールや仕組みを活用した経験はありますか？"
		}
	}
	switch {
	case strings.HasPrefix(code, "ENG"):
		return "プログラミングや技術に触れるのは好きですか？授業や趣味、独学で触れたことがあれば教えてください。"
	case strings.HasPrefix(code, "SALES"):
		return "営業で役立ちそうなITツールやアプリを使うことに興味はありますか？授業やアルバイトで使ったことがあれば教えてください。"
	case strings.HasPrefix(code, "MKT"):
		return "データやSNS分析など、デジタルを使って考えることに興味はありますか？授業や趣味で触れたことがあれば教えてください。"
	case strings.HasPrefix(code, "HR"):
		return "人事の仕事で役立ちそうなITツールや仕組みに興味はありますか？授業やアルバイトで使ったことがあれば教えてください。"
	case strings.HasPrefix(code, "FIN"):
		return "数字を扱う作業や表計算などのツールを使うのは好きですか？授業やアルバイトで使ったことがあれば教えてください。"
	case strings.HasPrefix(code, "CONS"):
		return "調べた情報をまとめるためにITツールやデータを使うことに興味はありますか？授業や課題での経験があれば教えてください。"
	default:
		return "身近なITツールやアプリを使って作業を効率化することに興味はありますか？授業やアルバイトで使った例があれば教えてください。"
	}
}

func (s *ChatService) getCategoryOrder(jobCategoryID uint) []string {
	defaultOrder := []string{
		"技術志向", "コミュニケーション能力", "リーダーシップ", "チームワーク",
		"問題解決力", "創造性・発想力", "計画性・実行力", "学習意欲・成長志向",
		"ストレス耐性・粘り強さ", "ビジネス思考・目標志向",
	}
	undecidedOrder := []string{
		"コミュニケーション能力", "学習意欲・成長志向", "問題解決力", "チームワーク",
		"ビジネス思考・目標志向", "計画性・実行力", "創造性・発想力", "ストレス耐性・粘り強さ",
		"リーダーシップ", "技術志向",
	}

	if jobCategoryID == 0 {
		return undecidedOrder
	}

	code := s.getJobCategoryCode(jobCategoryID)
	switch {
	case strings.HasPrefix(code, "ENG"):
		return []string{
			"技術志向", "問題解決力", "学習意欲・成長志向", "創造性・発想力",
			"計画性・実行力", "チームワーク", "コミュニケーション能力", "ストレス耐性・粘り強さ",
			"ビジネス思考・目標志向", "リーダーシップ",
		}
	case strings.HasPrefix(code, "SALES"):
		return []string{
			"コミュニケーション能力", "ビジネス思考・目標志向", "チームワーク", "ストレス耐性・粘り強さ",
			"計画性・実行力", "学習意欲・成長志向", "問題解決力", "リーダーシップ",
			"創造性・発想力", "技術志向",
		}
	case strings.HasPrefix(code, "MKT"):
		return []string{
			"創造性・発想力", "問題解決力", "コミュニケーション能力", "ビジネス思考・目標志向",
			"学習意欲・成長志向", "計画性・実行力", "チームワーク", "リーダーシップ",
			"ストレス耐性・粘り強さ", "技術志向",
		}
	case strings.HasPrefix(code, "HR"):
		return []string{
			"コミュニケーション能力", "チームワーク", "リーダーシップ", "学習意欲・成長志向",
			"計画性・実行力", "問題解決力", "ストレス耐性・粘り強さ", "ビジネス思考・目標志向",
			"創造性・発想力", "技術志向",
		}
	case strings.HasPrefix(code, "FIN"):
		return []string{
			"計画性・実行力", "問題解決力", "ビジネス思考・目標志向", "ストレス耐性・粘り強さ",
			"学習意欲・成長志向", "コミュニケーション能力", "チームワーク", "リーダーシップ",
			"創造性・発想力", "技術志向",
		}
	case strings.HasPrefix(code, "CONS"):
		return []string{
			"問題解決力", "コミュニケーション能力", "学習意欲・成長志向", "ビジネス思考・目標志向",
			"チームワーク", "リーダーシップ", "計画性・実行力", "ストレス耐性・粘り強さ",
			"創造性・発想力", "技術志向",
		}
	default:
		return defaultOrder
	}
}
