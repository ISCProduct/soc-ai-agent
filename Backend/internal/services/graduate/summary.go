package graduate

import (
	"sort"
	"strconv"
)

// Placement は集計に使う項目だけ。氏名は持たない。
type Placement struct {
	GraduationYear int
	SchoolName     string
	Industry       string
}

// Bucket は登録済み就職先の内訳。SharePercent は卒業生全体に対する内定率ではない。
type Bucket struct {
	Label        string `json:"label"`
	Count        int    `json:"count"`
	SharePercent int    `json:"share_percent"`
}

// Summary は登録されている就職先の件数と内訳。
// 卒業生全体の人数が無いので、内定率と進路決定率は計算しない。
type Summary struct {
	RecordedCount      int      `json:"recorded_count"`
	OfferRateAvailable bool     `json:"offer_rate_available"`
	Note               string   `json:"note"`
	ByYear             []Bucket `json:"by_year"`
	ByIndustry         []Bucket `json:"by_industry"`
	BySchool           []Bucket `json:"by_school"`
}

const rateUnavailableNote = "登録されている就職先の内訳です。卒業生全体の人数が無いため、内定率と進路決定率は計算していません。"

// Summarize は年度・業界・学校ごとの件数と、登録件数に占める割合を返す。
func Summarize(rows []Placement) Summary {
	out := Summary{
		RecordedCount:      len(rows),
		OfferRateAvailable: false,
		Note:               rateUnavailableNote,
		ByYear:             []Bucket{},
		ByIndustry:         []Bucket{},
		BySchool:           []Bucket{},
	}
	if len(rows) == 0 {
		return out
	}
	years := map[string]int{}
	industries := map[string]int{}
	schools := map[string]int{}
	for _, row := range rows {
		years[yearLabel(row.GraduationYear)]++
		industries[orUnset(row.Industry, "業界未設定")]++
		schools[orUnset(row.SchoolName, "学校未設定")]++
	}
	out.ByYear = buckets(years, len(rows))
	out.ByIndustry = buckets(industries, len(rows))
	out.BySchool = buckets(schools, len(rows))
	return out
}

func yearLabel(year int) string {
	if year <= 0 {
		return "年度未設定"
	}
	return strconv.Itoa(year) + "卒"
}

func orUnset(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func buckets(counts map[string]int, total int) []Bucket {
	out := make([]Bucket, 0, len(counts))
	for label, count := range counts {
		out = append(out, Bucket{
			Label:        label,
			Count:        count,
			SharePercent: (count*100 + total/2) / total,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	return out
}
