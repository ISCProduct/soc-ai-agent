package company

import (
	"Backend/domain/repository"
	"Backend/internal/companyfetch"
	"Backend/internal/models"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// CorrelationMapInitialChildLimit は初期表示で出す「起点に直接つながる子会社」の件数。
// 残りは FE が追加表示する。全件は DirectChildren に含める。
const CorrelationMapInitialChildLimit = 4

// CorrelationMapShortBusinessMaxRunes はカード用の短い事業説明の上限。
const CorrelationMapShortBusinessMaxRunes = 48

// 国税庁の法人種別コード（リソース定義書 k-resource-dl-ver4 の「法人種別」）。
// 相関図から除くのは、企業ではない次の2種別だけ。他のコード（301 株式会社 など）は推測で足さない。
const (
	CorporateKindNationalAgency  = "101" // 国の機関（行政機関・裁判所・国会の機関）
	CorporateKindLocalGovernment = "201" // 地方公共団体（普通地方公共団体・特別地方公共団体）
)

// wellKnownEmployeeThreshold は「よく知る企業」とみなす従業員数の下限。
// 登記が閉鎖された企業でも、よく知る企業なら相関図に残す（合併・社名変更の前の名前で知られているため）。
// 暫定値。運用しながら調整する前提で、定義はこの定数と isWellKnownCompany の1か所に集めている。
const wellKnownEmployeeThreshold = 1000

// isWellKnownCompany は「よく知る企業」か。上場している、または従業員数が閾値以上。
// 「よく知る企業」の定義を変えるときは、ここだけを直す。
func isWellKnownCompany(c *models.Company, market *models.CompanyMarketInfo) bool {
	if market != nil && market.IsListed {
		return true
	}
	return c.EmployeeCount >= wellKnownEmployeeThreshold
}

// isPublicBody は国の機関・地方公共団体か（企業ではない）。未確認（空）は false。
func isPublicBody(c *models.Company) bool {
	switch strings.TrimSpace(c.CorporateKind) {
	case CorporateKindNationalAgency, CorporateKindLocalGovernment:
		return true
	}
	return false
}

// isRegistryClosed は登記が閉鎖されているか。未確認・閉鎖なし（nil）は false。
func isRegistryClosed(c *models.Company) bool {
	return c.RegistryClosedOn != nil
}

// correlationExcluded は相関図に出さない企業か。
// 国の機関・地方公共団体、または「よく知る企業」でない閉鎖企業。未確認（registry_checked_at が NULL）は除外しない。
func correlationExcluded(c *models.Company, market *models.CompanyMarketInfo) bool {
	if c.RegistryCheckedAt == nil {
		return false
	}
	if isPublicBody(c) {
		return true
	}
	return isRegistryClosed(c) && !isWellKnownCompany(c, market)
}

// CorrelationMapNode は相関図カード／詳細に必要な企業情報。
type CorrelationMapNode struct {
	ID                  uint       `json:"id"`
	Name                string     `json:"name"`
	NameReading         string     `json:"name_reading"`
	NameReadingMissing  bool       `json:"name_reading_missing"`
	ShortBusiness       string     `json:"short_business"`
	ShortBusinessSource string     `json:"short_business_source,omitempty"` // main_business|description|""
	ShortBusinessKnown  bool       `json:"short_business_known"`
	MainBusiness        string     `json:"main_business"`
	Description         string     `json:"description"`
	Industry            string     `json:"industry,omitempty"`   // 業種。事業タグではない
	BusinessTags        []string   `json:"business_tags"`        // 事業・サービス分類。未整備時は空
	BusinessTagsStatus  string     `json:"business_tags_status"` // available|unavailable
	MarketType          string     `json:"market_type"`          // prime|standard|growth|unlisted|""
	MarketTypeKnown     bool       `json:"market_type_known"`    // false=未確認（非上場とは別）
	IsListed            bool       `json:"is_listed"`
	SourceURL           string     `json:"source_url,omitempty"`
	SourceFetchedAt     *time.Time `json:"source_fetched_at,omitempty"`
	RelationsFetchedAt  *time.Time `json:"relations_fetched_at,omitempty"`
	IsFocus             bool       `json:"is_focus"`
	// RegistryClosed は登記上は閉鎖されている企業（よく知る企業のため残している）。画面で「登記上は閉鎖」と注記する。
	RegistryClosed   bool   `json:"registry_closed"`
	RegistryClosedOn string `json:"registry_closed_on,omitempty"` // YYYY-MM-DD。不明なら省略
	DetailPath       string `json:"detail_path"`                  // FE の企業詳細への相対パス
}

// CorrelationMapCapitalEdge は資本関係（親→子）。subsidiary と affiliate を区別する。
type CorrelationMapCapitalEdge struct {
	ID               uint     `json:"id"`
	ParentID         uint     `json:"parent_id"`
	ChildID          uint     `json:"child_id"`
	RelationType     string   `json:"relation_type"` // capital_subsidiary|capital_affiliate|...
	RelationLabel    string   `json:"relation_label"`
	Description      string   `json:"description"`
	DescriptionKnown bool     `json:"description_known"` // 役割説明があるか（種別ラベルだけなら false）
	Ratio            *float64 `json:"ratio,omitempty"`
}

// CorrelationMapBusinessEdge は取引・協力関係。
// DirectionKnown は現状常に false。from/to は取得起点→相手であり、提供側→利用側ではない。
type CorrelationMapBusinessEdge struct {
	ID               uint   `json:"id"`
	FromID           uint   `json:"from_id"`
	ToID             uint   `json:"to_id"`
	DirectionKnown   bool   `json:"direction_known"`
	RelationType     string `json:"relation_type"`
	RelationLabel    string `json:"relation_label"`
	Description      string `json:"description"`
	DescriptionKnown bool   `json:"description_known"`
}

// CorrelationMapMeta は初期表示・追加表示・並び用の補助情報。
type CorrelationMapMeta struct {
	DirectParentIDs     []uint `json:"direct_parent_ids"`
	DirectChildIDs      []uint `json:"direct_child_ids"` // 読み仮名順
	DirectChildTotal    int    `json:"direct_child_total"`
	InitialChildLimit   int    `json:"initial_child_limit"`
	InitialChildIDs     []uint `json:"initial_child_ids"`
	RemainingChildCount int    `json:"remaining_child_count"`
	Truncated           bool   `json:"truncated"`
	MaxDepth            int    `json:"max_depth"`
	MaxNodes            int    `json:"max_nodes"`
}

// CompanyCorrelationMap は学生向け企業相関図の1レスポンス。
type CompanyCorrelationMap struct {
	FocusCompanyID uint                         `json:"focus_company_id"`
	Nodes          []CorrelationMapNode         `json:"nodes"`
	CapitalEdges   []CorrelationMapCapitalEdge  `json:"capital_edges"`
	BusinessEdges  []CorrelationMapBusinessEdge `json:"business_edges"`
	Meta           CorrelationMapMeta           `json:"meta"`
}

// CompanyCorrelationMapService は既存の関係グラフを、画面用の形に組み立てる。
type CompanyCorrelationMapService struct {
	companyRepo  repository.CompanyRepository
	relationRepo repository.CompanyRelationRepository
	graph        *CompanyRelationGraphService
}

func NewCompanyCorrelationMapService(
	companyRepo repository.CompanyRepository,
	relationRepo repository.CompanyRelationRepository,
) *CompanyCorrelationMapService {
	return &CompanyCorrelationMapService{
		companyRepo:  companyRepo,
		relationRepo: relationRepo,
		graph:        NewCompanyRelationGraphService(companyRepo, relationRepo),
	}
}

// Build は起点企業の相関図データを返す。
func (s *CompanyCorrelationMapService) Build(focusID uint) (*CompanyCorrelationMap, error) {
	base, err := s.graph.BuildGraph(focusID)
	if err != nil {
		return nil, err
	}

	nodeIDs := make([]uint, 0, len(base.Nodes))
	for _, n := range base.Nodes {
		nodeIDs = append(nodeIDs, n.ID)
	}
	companies, err := s.companyRepo.FindByIDs(nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("load companies: %w", err)
	}
	companyByID := map[uint]models.Company{}
	for _, c := range companies {
		companyByID[c.ID] = c
	}
	// 起点は FindByIDs に含まれない場合がある（実装差）ので保証する
	if _, ok := companyByID[focusID]; !ok {
		focus, err := s.companyRepo.FindByID(focusID)
		if err != nil {
			return nil, err
		}
		companyByID[focusID] = *focus
	}
	marketByID, err := s.relationRepo.GetMarketInfoByCompanyIDs(nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("load market info: %w", err)
	}

	focusCompany := companyByID[focusID]
	if !briefVisible(&focusCompany) || correlationExcluded(&focusCompany, marketByID[focusID]) {
		return nil, fmt.Errorf("company not found (id=%d): %w", focusID, repository.ErrNotFound)
	}

	relations, err := s.relationRepo.GetRelationsByCompanyIDs(nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("load relations: %w", err)
	}

	// 公開API向け: 国の機関・地方公共団体と、よく知る企業でない閉鎖企業もここで除く
	// （辺は下の nodeSet に無いノードにつながるものを出さないので、一緒に消える）。
	// 審査前ゲスト投稿をノードから除外（GetRelationsByCompanyIDs に
	// guestEntryVisibilityGuard が無いため、ここで多層防御する）。
	visibleIDs := make([]uint, 0, len(base.Nodes))
	for _, n := range base.Nodes {
		c, ok := companyByID[n.ID]
		if !ok || !briefVisible(&c) || correlationExcluded(&c, marketByID[n.ID]) {
			continue
		}
		visibleIDs = append(visibleIDs, n.ID)
	}

	out := &CompanyCorrelationMap{
		FocusCompanyID: focusID,
		Nodes:          make([]CorrelationMapNode, 0, len(visibleIDs)),
		CapitalEdges:   []CorrelationMapCapitalEdge{},
		BusinessEdges:  []CorrelationMapBusinessEdge{},
	}

	for _, id := range visibleIDs {
		c := companyByID[id]
		out.Nodes = append(out.Nodes, buildCorrelationNode(c, marketByID[id], id == focusID))
	}
	sortCorrelationNodes(out.Nodes)

	nodeSet := map[uint]struct{}{}
	for _, id := range visibleIDs {
		nodeSet[id] = struct{}{}
	}

	parentFreq := map[uint]int{}
	for _, rel := range relations {
		if !models.IsCapitalRelationType(rel.RelationType) || rel.ParentID == nil {
			continue
		}
		parentFreq[*rel.ParentID]++
	}

	capitalByPair := map[string]CorrelationMapCapitalEdge{}
	businessByPair := map[string]CorrelationMapBusinessEdge{}

	for _, rel := range relations {
		if models.IsCapitalRelationType(rel.RelationType) {
			if rel.ParentID == nil || rel.ChildID == nil {
				continue
			}
			if _, ok := nodeSet[*rel.ParentID]; !ok {
				continue
			}
			if _, ok := nodeSet[*rel.ChildID]; !ok {
				continue
			}
			desc := companyfetch.SanitizeRelationDescription(rel.Description)
			edge := CorrelationMapCapitalEdge{
				ID:               rel.ID,
				ParentID:         *rel.ParentID,
				ChildID:          *rel.ChildID,
				RelationType:     rel.RelationType,
				RelationLabel:    companyfetch.DefaultRelationDescription(rel.RelationType),
				Description:      desc,
				DescriptionKnown: desc != "",
				Ratio:            rel.Ratio,
			}
			key := undirectedPairKey(edge.ParentID, edge.ChildID, edge.RelationType)
			if prev, ok := capitalByPair[key]; ok {
				capitalByPair[key] = preferCapitalEdge(prev, edge, parentFreq, marketByID)
			} else {
				capitalByPair[key] = edge
			}
			continue
		}

		edge, ok := buildCorrelationBusinessEdge(rel, nodeSet)
		if !ok {
			continue
		}
		key := undirectedPairKey(edge.FromID, edge.ToID, edge.RelationType)
		if _, seen := businessByPair[key]; seen {
			// 鏡像の取引行は片方だけ残す（向きは信頼しない）
			continue
		}
		businessByPair[key] = edge
	}

	var directParents, directChildren []uint
	for _, edge := range capitalByPair {
		out.CapitalEdges = append(out.CapitalEdges, edge)
		if edge.ChildID == focusID {
			directParents = append(directParents, edge.ParentID)
		}
		if edge.ParentID == focusID {
			directChildren = append(directChildren, edge.ChildID)
		}
	}
	for _, edge := range businessByPair {
		out.BusinessEdges = append(out.BusinessEdges, edge)
	}
	sort.SliceStable(out.CapitalEdges, func(i, j int) bool {
		if out.CapitalEdges[i].ParentID != out.CapitalEdges[j].ParentID {
			return out.CapitalEdges[i].ParentID < out.CapitalEdges[j].ParentID
		}
		return out.CapitalEdges[i].ChildID < out.CapitalEdges[j].ChildID
	})
	sort.SliceStable(out.BusinessEdges, func(i, j int) bool {
		if out.BusinessEdges[i].FromID != out.BusinessEdges[j].FromID {
			return out.BusinessEdges[i].FromID < out.BusinessEdges[j].FromID
		}
		return out.BusinessEdges[i].ToID < out.BusinessEdges[j].ToID
	})

	directParents = uniqueSortedIDs(directParents, companyByID)
	directChildren = uniqueSortedIDs(directChildren, companyByID)
	initial := directChildren
	if len(initial) > CorrelationMapInitialChildLimit {
		initial = initial[:CorrelationMapInitialChildLimit]
	}

	out.Meta = CorrelationMapMeta{
		DirectParentIDs:     directParents,
		DirectChildIDs:      directChildren,
		DirectChildTotal:    len(directChildren),
		InitialChildLimit:   CorrelationMapInitialChildLimit,
		InitialChildIDs:     initial,
		RemainingChildCount: max(0, len(directChildren)-len(initial)),
		Truncated:           base.Truncated,
		MaxDepth:            RelationGraphMaxDepth,
		MaxNodes:            RelationGraphMaxNodes,
	}
	return out, nil
}

// CompareBusiness は起点と選択企業の事業説明を並べて返す（専用比較文は持たない）。
func (s *CompanyCorrelationMapService) CompareBusiness(focusID, selectedID uint) (map[string]any, error) {
	ids := []uint{focusID, selectedID}
	companies, err := s.companyRepo.FindByIDs(ids)
	if err != nil {
		return nil, err
	}
	byID := map[uint]models.Company{}
	for _, c := range companies {
		byID[c.ID] = c
	}
	focus, ok1 := byID[focusID]
	selected, ok2 := byID[selectedID]
	// 公開API。審査前のゲスト投稿企業と、相関図から除く企業は、Build と同じ条件で隠す。
	if !ok1 || !ok2 || !briefVisible(&focus) || !briefVisible(&selected) {
		return nil, fmt.Errorf("company not found: %w", repository.ErrNotFound)
	}
	// 市場情報（上場）は、閉鎖企業の判定に要るときだけ引く。
	var marketByID map[uint]*models.CompanyMarketInfo
	if isRegistryClosed(&focus) || isRegistryClosed(&selected) {
		if marketByID, err = s.relationRepo.GetMarketInfoByCompanyIDs(ids); err != nil {
			return nil, fmt.Errorf("load market info: %w", err)
		}
	}
	if correlationExcluded(&focus, marketByID[focusID]) || correlationExcluded(&selected, marketByID[selectedID]) {
		return nil, fmt.Errorf("company not found: %w", repository.ErrNotFound)
	}
	fs, fsrc, fKnown := DeriveShortBusiness(focus.MainBusiness, focus.Description)
	ss, ssrc, sKnown := DeriveShortBusiness(selected.MainBusiness, selected.Description)
	return map[string]any{
		"focus_company_id":    focusID,
		"selected_company_id": selectedID,
		"focus": map[string]any{
			"id":                    focus.ID,
			"name":                  focus.Name,
			"short_business":        fs,
			"short_business_source": fsrc,
			"short_business_known":  fKnown,
			"main_business":         focus.MainBusiness,
			"description":           focus.Description,
		},
		"selected": map[string]any{
			"id":                    selected.ID,
			"name":                  selected.Name,
			"short_business":        ss,
			"short_business_source": ssrc,
			"short_business_known":  sKnown,
			"main_business":         selected.MainBusiness,
			"description":           selected.Description,
		},
		"compare_text_available": false, // 組み合わせ専用の比較文は未実装。FEは両社説明を並べる
	}, nil
}

func buildCorrelationNode(c models.Company, market *models.CompanyMarketInfo, isFocus bool) CorrelationMapNode {
	reading := strings.TrimSpace(c.NameReading)
	short, src, known := DeriveShortBusiness(c.MainBusiness, c.Description)
	n := CorrelationMapNode{
		ID:                  c.ID,
		Name:                c.Name,
		NameReading:         reading,
		NameReadingMissing:  reading == "",
		ShortBusiness:       short,
		ShortBusinessSource: src,
		ShortBusinessKnown:  known,
		MainBusiness:        c.MainBusiness,
		Description:         c.Description,
		Industry:            c.Industry,
		BusinessTags:        []string{},
		BusinessTagsStatus:  "unavailable", // 事業タグ専用データは未整備（industry は流用しない）
		IsFocus:             isFocus,
		SourceURL:           c.SourceURL,
		SourceFetchedAt:     c.SourceFetchedAt,
		RelationsFetchedAt:  c.RelationsFetchedAt,
		DetailPath:          fmt.Sprintf("/company/%d", c.ID),
	}
	if isRegistryClosed(&c) {
		n.RegistryClosed = true
		n.RegistryClosedOn = c.RegistryClosedOn.Format("2006-01-02")
	}
	if market != nil {
		n.MarketTypeKnown = true
		n.IsListed = market.IsListed
		n.MarketType = normalizeMarketType(market.MarketType, market.IsListed)
	}
	return n
}

func buildCorrelationBusinessEdge(rel models.CompanyRelation, nodeSet map[uint]struct{}) (CorrelationMapBusinessEdge, bool) {
	if rel.FromID == nil || rel.ToID == nil {
		return CorrelationMapBusinessEdge{}, false
	}
	if _, ok := nodeSet[*rel.FromID]; !ok {
		return CorrelationMapBusinessEdge{}, false
	}
	if _, ok := nodeSet[*rel.ToID]; !ok {
		return CorrelationMapBusinessEdge{}, false
	}
	desc := companyfetch.SanitizeRelationDescription(rel.Description)
	return CorrelationMapBusinessEdge{
		ID:               rel.ID,
		FromID:           *rel.FromID,
		ToID:             *rel.ToID,
		DirectionKnown:   false, // persist は取得起点→相手。提供/利用の意味は持たない
		RelationType:     rel.RelationType,
		RelationLabel:    companyfetch.DefaultRelationDescription(rel.RelationType),
		Description:      desc,
		DescriptionKnown: desc != "",
	}, true
}

func undirectedPairKey(a, b uint, relationType string) string {
	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}
	return fmt.Sprintf("%d-%d-%s", lo, hi, relationType)
}

// preferCapitalEdge は鏡像の資本行から1本を残す。
// 親としての出現回数 → 出資比率 → 上場親 → 小さい parent_id の順。
func preferCapitalEdge(
	a, b CorrelationMapCapitalEdge,
	parentFreq map[uint]int,
	marketByID map[uint]*models.CompanyMarketInfo,
) CorrelationMapCapitalEdge {
	if parentFreq[a.ParentID] != parentFreq[b.ParentID] {
		if parentFreq[a.ParentID] > parentFreq[b.ParentID] {
			return a
		}
		return b
	}
	ra, rb := 0.0, 0.0
	if a.Ratio != nil {
		ra = *a.Ratio
	}
	if b.Ratio != nil {
		rb = *b.Ratio
	}
	if ra != rb {
		if ra > rb {
			return a
		}
		return b
	}
	aListed := marketByID[a.ParentID] != nil && marketByID[a.ParentID].IsListed
	bListed := marketByID[b.ParentID] != nil && marketByID[b.ParentID].IsListed
	if aListed != bListed {
		if aListed {
			return a
		}
		return b
	}
	if a.ParentID <= b.ParentID {
		return a
	}
	return b
}

// DeriveShortBusiness はカード用の短い事業説明を既存文から作る。
// 専用 short 文が無い前提で、main_business → description の順に使う。
func DeriveShortBusiness(mainBusiness, description string) (text, source string, known bool) {
	for _, cand := range []struct {
		raw, src string
	}{
		{mainBusiness, "main_business"},
		{description, "description"},
	} {
		raw := strings.TrimSpace(cand.raw)
		if raw == "" {
			continue
		}
		trimmed := trimToShortBusiness(raw)
		if trimmed == "" {
			continue
		}
		return trimmed, cand.src, true
	}
	return "", "", false
}

func trimToShortBusiness(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	// 句点で一文目を優先
	for _, sep := range []string{"。", "．", ". ", "\n"} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
			break
		}
	}
	if utf8.RuneCountInString(s) <= CorrelationMapShortBusinessMaxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:CorrelationMapShortBusinessMaxRunes]) + "…"
}

func sortCorrelationNodes(nodes []CorrelationMapNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return correlationSortKey(nodes[i]) < correlationSortKey(nodes[j])
	})
}

func correlationSortKey(n CorrelationMapNode) string {
	if r := strings.TrimSpace(n.NameReading); r != "" {
		return strings.ToLower(r)
	}
	return strings.ToLower(strings.TrimSpace(n.Name))
}

func uniqueSortedIDs(ids []uint, companies map[uint]models.Company) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := companies[out[i]], companies[out[j]]
		ka := strings.ToLower(strings.TrimSpace(a.NameReading))
		if ka == "" {
			ka = strings.ToLower(strings.TrimSpace(a.Name))
		}
		kb := strings.ToLower(strings.TrimSpace(b.NameReading))
		if kb == "" {
			kb = strings.ToLower(strings.TrimSpace(b.Name))
		}
		if ka == kb {
			return out[i] < out[j]
		}
		return ka < kb
	})
	return out
}
