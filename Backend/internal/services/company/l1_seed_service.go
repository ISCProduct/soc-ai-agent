package company

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strings"
)

// L1SeedRow はカタログシード CSV の1行。
type L1SeedRow struct {
	Name       string
	Industry   string
	Segment    string // core | sme_si | extended
	WebsiteURL string
	Publish    bool
}

// L1SeedResult は投入結果。
type L1SeedResult struct {
	Total   int      `json:"total"`
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// ParseL1SeedCSV は name,industry,segment,website_url,publish 形式を読む。
func ParseL1SeedCSV(r io.Reader) ([]L1SeedRow, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty csv")
	}
	start := 0
	header := map[string]int{}
	if looksLikeL1Header(records[0]) {
		for i, h := range records[0] {
			header[strings.ToLower(strings.TrimSpace(h))] = i
		}
		start = 1
	} else {
		header = map[string]int{"name": 0, "industry": 1, "segment": 2, "website_url": 3, "publish": 4}
	}
	nameIdx, ok := header["name"]
	if !ok {
		return nil, fmt.Errorf("csv must include name column")
	}
	var rows []L1SeedRow
	for i := start; i < len(records); i++ {
		rec := records[i]
		if len(rec) <= nameIdx {
			continue
		}
		name := strings.TrimSpace(rec[nameIdx])
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		row := L1SeedRow{Name: name}
		if idx, ok := header["industry"]; ok && idx < len(rec) {
			row.Industry = strings.TrimSpace(rec[idx])
		}
		if idx, ok := header["segment"]; ok && idx < len(rec) {
			row.Segment = normalizeL1Segment(rec[idx])
		} else {
			row.Segment = "extended"
		}
		if idx, ok := header["website_url"]; ok && idx < len(rec) {
			row.WebsiteURL = strings.TrimSpace(rec[idx])
		}
		if idx, ok := header["publish"]; ok && idx < len(rec) {
			row.Publish = parseBoolish(rec[idx])
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func looksLikeL1Header(rec []string) bool {
	for _, c := range rec {
		if strings.EqualFold(strings.TrimSpace(c), "name") {
			return true
		}
	}
	return false
}

func normalizeL1Segment(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "core", "sme_si", "sme-si", "中小si", "si":
		if s == "sme-si" || s == "中小si" || s == "si" {
			return "sme_si"
		}
		return s
	case "extended", "ext", "":
		return "extended"
	default:
		return "extended"
	}
}

func parseBoolish(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "published", "publish":
		return true
	default:
		return false
	}
}

// ImportL1Seed はシード行を companies に upsert する（Search しない）。
func ImportL1Seed(repo repository.CompanyRepository, rows []L1SeedRow) (*L1SeedResult, error) {
	if repo == nil {
		return nil, fmt.Errorf("company repo is nil")
	}
	res := &L1SeedResult{Total: len(rows)}
	for _, row := range rows {
		existing, err := repo.FindByName(row.Name)
		if err == nil && existing != nil {
			changed := false
			if row.Industry != "" && existing.Industry == "" {
				existing.Industry = row.Industry
				changed = true
			}
			if row.WebsiteURL != "" && existing.WebsiteURL == "" {
				existing.WebsiteURL = row.WebsiteURL
				changed = true
			}
			if row.Publish && existing.DataStatus != "published" {
				existing.DataStatus = "published"
				existing.IsActive = true
				existing.IsProvisional = false
				changed = true
			}
			// segment を industry 接頭辞で残す（専用カラムなし）
			if row.Segment == "core" || row.Segment == "sme_si" {
				tag := "[L1:" + row.Segment + "]"
				if !strings.Contains(existing.Description, tag) {
					existing.Description = strings.TrimSpace(tag + " " + existing.Description)
					changed = true
				}
			}
			if changed {
				if err := repo.Update(existing); err != nil {
					res.Errors = append(res.Errors, fmt.Sprintf("%s: update %v", row.Name, err))
					continue
				}
				res.Updated++
			} else {
				res.Skipped++
			}
			continue
		}
		c := &models.Company{
			Name:          row.Name,
			Industry:      row.Industry,
			WebsiteURL:    row.WebsiteURL,
			SourceType:    "manual",
			IsActive:      true,
			IsProvisional: !row.Publish,
			DataStatus:    "draft",
		}
		if row.Segment == "core" || row.Segment == "sme_si" {
			c.Description = "[L1:" + row.Segment + "]"
		}
		if row.Publish {
			c.DataStatus = "published"
			c.IsProvisional = false
		}
		if row.Industry == "" && row.Segment == "sme_si" {
			c.Industry = "情報サービス・SI"
		}
		if err := repo.Create(c); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: create %v", row.Name, err))
			continue
		}
		res.Created++
	}
	log.Printf("l1_seed: total=%d created=%d updated=%d skipped=%d errors=%d",
		res.Total, res.Created, res.Updated, res.Skipped, len(res.Errors))
	return res, nil
}
