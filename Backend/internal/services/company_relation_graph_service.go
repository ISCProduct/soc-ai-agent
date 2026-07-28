package services

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"fmt"
)

// 親会社→子会社→孫会社のように資本関係を辿る際の安全弁。
// 深さ・ノード数のどちらかに達したら打ち切り、無限ループや巨大レスポンスを防ぐ。
const (
	RelationGraphMaxDepth = 4
	RelationGraphMaxNodes = 60
)

// RelationGraphNode はグラフ上の1企業ノード。
type RelationGraphNode struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	MarketType string `json:"market_type,omitempty"`
	IsListed   bool   `json:"is_listed"`
	IsFocus    bool   `json:"is_focus"`
}

// RelationGraphCapitalEdge は資本関係の1辺（親→子）。
type RelationGraphCapitalEdge struct {
	ParentID     uint     `json:"parent_id"`
	ChildID      uint     `json:"child_id"`
	RelationType string   `json:"relation_type"`
	Ratio        *float64 `json:"ratio,omitempty"`
}

// RelationGraphBusinessEntry は起点企業から見た取引関係1件（多段階化はしない）。
type RelationGraphBusinessEntry struct {
	CompanyID    uint   `json:"company_id"`
	Name         string `json:"name"`
	RelationType string `json:"relation_type"`
	Description  string `json:"description,omitempty"`
}

// CompanyRelationGraph は起点企業を中心にした資本関係グラフ + 直接の取引関係。
type CompanyRelationGraph struct {
	CompanyID         uint                         `json:"company_id"`
	Nodes             []RelationGraphNode          `json:"nodes"`
	CapitalEdges      []RelationGraphCapitalEdge   `json:"capital_edges"`
	BusinessRelations []RelationGraphBusinessEntry `json:"business_relations"`
	// Truncated はノード数上限に達し、一部の資本関係を打ち切ったことを示す。
	Truncated bool `json:"truncated"`
}

// CompanyRelationGraphService は起点企業から資本関係を多段階（親会社→子会社→孫会社等）に辿り、
// 相関図表示用のグラフ構造を組み立てる。
type CompanyRelationGraphService struct {
	companyRepo  repository.CompanyRepository
	relationRepo repository.CompanyRelationRepository
}

func NewCompanyRelationGraphService(companyRepo repository.CompanyRepository, relationRepo repository.CompanyRelationRepository) *CompanyRelationGraphService {
	return &CompanyRelationGraphService{companyRepo: companyRepo, relationRepo: relationRepo}
}

// BuildGraph は companyID を起点に資本関係を BFS で辿ったグラフを返す。
// 取引関係は起点企業から直接分のみ付加する（取引先の取引先までは辿らない）。
func (s *CompanyRelationGraphService) BuildGraph(companyID uint) (*CompanyRelationGraph, error) {
	graph := &CompanyRelationGraph{CompanyID: companyID}
	nodeSeen := map[uint]struct{}{}
	capitalEdgeSeen := map[string]struct{}{}
	businessSeen := map[string]struct{}{}

	// addNode は id をグラフに追加する。既に追加済みなら true、上限到達で追加できなければ false を返す。
	addNode := func(id uint) (bool, error) {
		if _, ok := nodeSeen[id]; ok {
			return true, nil
		}
		if len(nodeSeen) >= RelationGraphMaxNodes {
			graph.Truncated = true
			return false, nil
		}
		company, err := s.companyRepo.FindByID(id)
		if err != nil {
			return false, fmt.Errorf("company not found (id=%d): %w", id, err)
		}
		marketType := ""
		isListed := false
		if info, err := s.relationRepo.GetMarketInfoByCompanyID(id); err == nil && info != nil {
			marketType = info.MarketType
			isListed = info.IsListed
		}
		nodeSeen[id] = struct{}{}
		graph.Nodes = append(graph.Nodes, RelationGraphNode{
			ID:         id,
			Name:       company.Name,
			MarketType: marketType,
			IsListed:   isListed,
			IsFocus:    id == companyID,
		})
		return true, nil
	}

	if _, err := addNode(companyID); err != nil {
		return nil, err
	}

	type queueItem struct {
		id    uint
		depth int
	}
	queued := map[uint]struct{}{companyID: {}}
	queue := []queueItem{{id: companyID, depth: 0}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if len(nodeSeen) >= RelationGraphMaxNodes {
			graph.Truncated = true
			break
		}

		relations, err := s.relationRepo.GetRelationsByCompanyID(item.id)
		if err != nil {
			return nil, err
		}

		for _, rel := range relations {
			if models.IsCapitalRelationType(rel.RelationType) {
				if rel.ParentID == nil || rel.ChildID == nil {
					continue
				}
				edgeKey := fmt.Sprintf("%d-%d-%s", *rel.ParentID, *rel.ChildID, rel.RelationType)
				if _, ok := capitalEdgeSeen[edgeKey]; !ok {
					capitalEdgeSeen[edgeKey] = struct{}{}
					graph.CapitalEdges = append(graph.CapitalEdges, RelationGraphCapitalEdge{
						ParentID:     *rel.ParentID,
						ChildID:      *rel.ChildID,
						RelationType: rel.RelationType,
						Ratio:        rel.Ratio,
					})
				}
				if item.depth >= RelationGraphMaxDepth {
					for _, nextID := range []uint{*rel.ParentID, *rel.ChildID} {
						if nextID != item.id {
							if _, ok := nodeSeen[nextID]; !ok {
								graph.Truncated = true
							}
						}
					}
					continue
				}
				for _, nextID := range []uint{*rel.ParentID, *rel.ChildID} {
					if nextID == item.id {
						continue
					}
					added, err := addNode(nextID)
					if err != nil {
						return nil, err
					}
					if !added {
						continue
					}
					if _, ok := queued[nextID]; !ok {
						queued[nextID] = struct{}{}
						queue = append(queue, queueItem{id: nextID, depth: item.depth + 1})
					}
				}
				continue
			}

			// 取引関係は起点企業から直接分のみ採用する。
			if item.id != companyID {
				continue
			}
			var partnerID uint
			switch {
			case rel.FromID != nil && *rel.FromID == companyID && rel.ToID != nil:
				partnerID = *rel.ToID
			case rel.ToID != nil && *rel.ToID == companyID && rel.FromID != nil:
				partnerID = *rel.FromID
			default:
				continue
			}
			key := fmt.Sprintf("%d-%s", partnerID, rel.RelationType)
			if _, ok := businessSeen[key]; ok {
				continue
			}
			partner, err := s.companyRepo.FindByID(partnerID)
			if err != nil {
				continue
			}
			businessSeen[key] = struct{}{}
			graph.BusinessRelations = append(graph.BusinessRelations, RelationGraphBusinessEntry{
				CompanyID:    partnerID,
				Name:         partner.Name,
				RelationType: rel.RelationType,
				Description:  rel.Description,
			})
		}
	}

	return graph, nil
}
