package chat

import (
	"strings"
)

func (s *ChatService) fallbackQuestionForCategory(category string, jobCategoryID uint, targetLevel string) string {
	switch category {
	case "技術志向":
		return s.techInterestQuestion(jobCategoryID, targetLevel)
	case "コミュニケーション力":
		if targetLevel == "中途" {
			return "業務で関係者と調整した経験はありますか？どんな場面で、どのように進めましたか？"
		}
		return "グループワークで、自分の考えをどのように伝えていますか？"
	case "リーダーシップ志向":
		if targetLevel == "中途" {
			return "業務でチームや案件をリードした経験はありますか？どのように進めましたか？"
		}
		return "グループで何かをまとめた経験はありますか？どんな場面でしたか？"
	case "チームワーク志向":
		if targetLevel == "中途" {
			return "チームで協力して成果を出した経験はありますか？あなたの役割も教えてください。"
		}
		return "サークルや授業で、チームで取り組んだ経験はありますか？どんな役割でしたか？"
	case "安定志向":
		if targetLevel == "中途" {
			return "働き方について、変化の多い環境と落ち着いた環境のどちらが合うと感じますか？"
		}
		return "働く環境について、腰を据えて続けられることと新しい変化のどちらを大事にしたいですか？"
	case "創造性志向":
		if targetLevel == "中途" {
			return "業務で改善や工夫を提案した経験はありますか？どんな内容でしたか？"
		}
		return "新しいアイデアを出した経験はありますか？どんな工夫をしましたか？"
	case "細部志向":
		if targetLevel == "中途" {
			return "業務で計画を立てて実行した経験を教えてください。どのように進めましたか？"
		}
		return "何かを計画して実行した経験を教えてください。どのように進めましたか？"
	case "成長志向":
		if targetLevel == "中途" {
			return "業務に役立てるために学んだことはありますか？直近の例があれば教えてください。"
		}
		return "新しいことを学ぶとき、どうやって学習を進めますか？直近で学んだことはありますか？"
	case "チャレンジ志向":
		if targetLevel == "中途" {
			return "業務で困難に直面したとき、どのように乗り越えましたか？具体例があれば教えてください。"
		}
		return "困難に直面したとき、どのように乗り越えましたか？具体例があれば教えてください。"
	case "ワークライフバランス":
		if targetLevel == "中途" {
			return "仕事とプライベートの時間配分について、どんな働き方が理想ですか？"
		}
		return "仕事とプライベートの過ごし方について、どんなバランスが理想ですか？"
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
	case "コミュニケーション力":
		if targetLevel == "中途" {
			return []string{
				"業務で相手に説明するとき、意識していることは何ですか？",
				"相手に合わせて説明の仕方を変えることはありますか？",
			}
		}
		return []string{
			"人に説明するとき、意識していることは何ですか？",
			"授業やサークルで発表した経験はありますか？",
		}
	case "リーダーシップ志向":
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
	case "チームワーク志向":
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
	case "安定志向":
		if targetLevel == "中途" {
			return []string{
				"変化の多い環境と、腰を据えて取り組める環境のどちらが合いますか？",
				"長く続けられる働き方として、大事にしたいことは何ですか？",
			}
		}
		return []string{
			"変化の多い環境と落ち着いた環境、どちらが自分に合うと思いますか？",
			"長く働くうえで大事にしたいことは何ですか？",
		}
	case "創造性志向":
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
	case "細部志向":
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
	case "成長志向":
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
	case "チャレンジ志向":
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
	case "ワークライフバランス":
		if targetLevel == "中途" {
			return []string{
				"仕事とプライベートの時間配分は、どのくらいが理想ですか？",
				"働き方で譲れない条件はありますか？",
			}
		}
		return []string{
			"仕事とプライベートのバランスについて、どんな働き方が理想ですか？",
			"働くうえで大事にしたい時間の使い方はありますか？",
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
	// 正典(domain/valueobject/match.go)の10種で並べる（#929）。
	//
	// 以前はここが正典とは別の10分類を返していたため、
	// chat_question_predefined.go の絞り込み(q.Category != prioritizeCategory)が
	// 技術志向以外で一致せず、事前定義質問(11件)が丸ごと使われないまま
	// 毎回AI生成にフォールバックしていた。未評価カテゴリの判定も
	// scoreMap(正典キー)と突き合わないため機能していなかった。
	//
	// 職種ごとの優先順位という元の意図は保ち、写像で重複した分と
	// 抜けていた安定志向・ワークライフバランスを補って10種を網羅する。
	undecidedOrder := []string{
		"コミュニケーション力", "成長志向", "技術志向", "チームワーク志向",
		"細部志向", "創造性志向", "チャレンジ志向", "リーダーシップ志向",
		"安定志向", "ワークライフバランス",
	}

	if jobCategoryID == 0 {
		return undecidedOrder
	}
	return categoryOrderForJobCode(s.getJobCategoryCode(jobCategoryID))
}

// categoryOrderForJobCode は職種コードごとの優先順位を返す純関数。
// DBに触らないので、正典10種を網羅していることをテストで直接検証できる（#929）。
func categoryOrderForJobCode(code string) []string {
	defaultOrder := []string{
		"技術志向", "コミュニケーション力", "リーダーシップ志向", "チームワーク志向",
		"創造性志向", "細部志向", "成長志向", "チャレンジ志向",
		"安定志向", "ワークライフバランス",
	}
	switch {
	case strings.HasPrefix(code, "ENG"):
		return []string{
			"技術志向", "成長志向", "創造性志向", "細部志向",
			"チームワーク志向", "コミュニケーション力", "チャレンジ志向", "リーダーシップ志向",
			"安定志向", "ワークライフバランス",
		}
	case strings.HasPrefix(code, "SALES"):
		return []string{
			"コミュニケーション力", "成長志向", "チームワーク志向", "チャレンジ志向",
			"細部志向", "技術志向", "リーダーシップ志向", "創造性志向",
			"安定志向", "ワークライフバランス",
		}
	case strings.HasPrefix(code, "MKT"):
		return []string{
			"創造性志向", "技術志向", "コミュニケーション力", "成長志向",
			"細部志向", "チームワーク志向", "リーダーシップ志向", "チャレンジ志向",
			"安定志向", "ワークライフバランス",
		}
	case strings.HasPrefix(code, "HR"):
		return []string{
			"コミュニケーション力", "チームワーク志向", "リーダーシップ志向", "成長志向",
			"細部志向", "技術志向", "チャレンジ志向", "創造性志向",
			"安定志向", "ワークライフバランス",
		}
	case strings.HasPrefix(code, "FIN"):
		// 金融は安定志向が職種適性として効くため上位に置く。
		return []string{
			"細部志向", "安定志向", "技術志向", "成長志向",
			"チャレンジ志向", "コミュニケーション力", "チームワーク志向", "リーダーシップ志向",
			"創造性志向", "ワークライフバランス",
		}
	case strings.HasPrefix(code, "CONS"):
		return []string{
			"技術志向", "コミュニケーション力", "成長志向", "チームワーク志向",
			"リーダーシップ志向", "細部志向", "チャレンジ志向", "創造性志向",
			"安定志向", "ワークライフバランス",
		}
	default:
		return defaultOrder
	}
}
