'use client';

import { useCallback, useMemo, useState, useEffect } from 'react';
import ReactFlow, {
    Node,
    Edge,
    Controls,
    Background,
    MiniMap,
    useNodesState,
    useEdgesState,
    MarkerType,
} from 'reactflow';
import 'reactflow/dist/style.css';
import { Box, Typography, Chip } from '@mui/material';
import {
    fetchCompanyRelations,
    fetchCompanyMarketInfo,
    marketColors,
    marketLabels,
    type CapitalRelation,
    type CompanyMarketInfo,
    type Company,
    type MarketType,
} from '@/lib/company-data';
import { formatRelationLabel } from '@/lib/relation-labels';
import { layoutBusinessGraph, layoutCapitalGraphFromEdges } from '@/lib/relation-graph';
import { edgeTypes } from '@/components/diagram/RelationEdge';

type DiagramType = 'capital' | 'business';

interface CompanyDiagramProps {
    companyId: number;
    diagramType: DiagramType;
}

export default function CompanyDiagram({ companyId, diagramType }: CompanyDiagramProps) {
    const [relations, setRelations] = useState<CapitalRelation[]>([]);
    const [marketInfo, setMarketInfo] = useState<CompanyMarketInfo[]>([]);
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState<string | null>(null);

    useEffect(() => {
        async function loadData() {
            setLoading(true);
            setLoadError(null);
            try {
                const [relationsData, marketData] = await Promise.all([
                    fetchCompanyRelations(),
                    fetchCompanyMarketInfo()
                ]);
                setRelations(relationsData);
                setMarketInfo(marketData);
            } catch (error) {
                setLoadError(error instanceof Error ? error.message : 'データの取得に失敗しました');
                setRelations([]);
                setMarketInfo([]);
            } finally {
                setLoading(false);
            }
        }
        void loadData();
    }, []);

    const getMarketType = useCallback((compId: number): MarketType => {
        const info = marketInfo.find(m => m.company_id === compId);
        return info?.market_type || 'unlisted';
    }, [marketInfo]);

    const getCompanyName = useCallback((compId: number): string => {
        // 関係データから企業名を取得
        for (const rel of relations) {
            if (rel.parent?.id === compId) return rel.parent.name;
            if (rel.child?.id === compId) return rel.child.name;
            if (rel.from?.id === compId) return rel.from.name;
            if (rel.to?.id === compId) return rel.to.name;
        }
        return `企業 ${compId}`;
    }, [relations]);

    const createCapitalNodes = useCallback((focusCompanyId: number): Node[] => {
        const relatedIds = new Set([focusCompanyId]);

        // 資本関係のあるIDを収集
        relations.forEach(rel => {
            if (rel.relation_type.startsWith('capital')) {
                if (rel.parent_id === focusCompanyId || rel.child_id === focusCompanyId) {
                    if (rel.parent_id) relatedIds.add(rel.parent_id);
                    if (rel.child_id) relatedIds.add(rel.child_id);
                }
            }
        });

        // 親会社も追加
        relations.forEach(rel => {
            if (rel.relation_type.startsWith('capital') && rel.child_id && relatedIds.has(rel.child_id)) {
                if (rel.parent_id) relatedIds.add(rel.parent_id);
            }
        });

        // #1022: 親→子の向きで固定配置していた独自ロジックを廃止し、business タブと同様に
        // 起点企業からのBFS距離(layoutCapitalGraphFromEdges)で配置する。起点が子会社でも
        // 親会社が常に上に固定されず、起点企業が常に最上位に来る。
        const involvedCompanies = Array.from(relatedIds);
        const capitalRelations = relations.filter(rel => rel.relation_type.startsWith('capital'));
        const positions = layoutCapitalGraphFromEdges(focusCompanyId, involvedCompanies, capitalRelations);

        return involvedCompanies.map((compId) => {
            const isFocusCompany = compId === focusCompanyId;
            const marketType = getMarketType(compId);
            const pos = positions.get(compId) ?? { x: 0, y: 0 };

            return {
                id: String(compId),
                type: 'default',
                position: { x: pos.x, y: pos.y },
                data: {
                    label: (
                        <Box sx={{ textAlign: 'center', p: 1 }}>
                            <Typography
                                variant="body2"
                                sx={{ fontWeight: isFocusCompany ? 'bold' : 'normal', mb: 0.5 }}
                            >
                                {getCompanyName(compId)}
                            </Typography>
                            <Chip
                                label={marketLabels[marketType]}
                                size="small"
                                sx={{
                                    bgcolor: marketColors[marketType],
                                    color: 'white',
                                    fontSize: '10px',
                                    height: '20px',
                                }}
                            />
                        </Box>
                    ),
                },
                style: {
                    background: isFocusCompany ? '#FFF3CD' : '#fff',
                    border: `3px solid ${isFocusCompany ? '#FFC107' : marketColors[marketType]}`,
                    borderRadius: '8px',
                    padding: '10px',
                    minWidth: '200px',
                    boxShadow: isFocusCompany ? '0 4px 12px rgba(255, 193, 7, 0.3)' : undefined,
                },
            };
        });
    }, [relations, getMarketType, getCompanyName]);

    const createCapitalEdges = useCallback((focusCompanyId: number): Edge[] => {
        const relatedIds = new Set([focusCompanyId]);

        relations.forEach(rel => {
            if (rel.relation_type.startsWith('capital')) {
                if (rel.parent_id === focusCompanyId || rel.child_id === focusCompanyId) {
                    if (rel.parent_id) relatedIds.add(rel.parent_id);
                    if (rel.child_id) relatedIds.add(rel.child_id);
                }
            }
        });

        relations.forEach(rel => {
            if (rel.relation_type.startsWith('capital') && rel.child_id && relatedIds.has(rel.child_id)) {
                if (rel.parent_id) relatedIds.add(rel.parent_id);
            }
        });

        return relations
            .filter(rel =>
                rel.relation_type.startsWith('capital') &&
                rel.parent_id && rel.child_id &&
                relatedIds.has(rel.parent_id) && relatedIds.has(rel.child_id)
            )
            .map((rel, idx) => ({
                id: `capital-${idx}`,
                source: String(rel.parent_id),
                target: String(rel.child_id),
                type: 'custom',
                label: rel.ratio ? `${rel.ratio}%` : '',
                animated: false,
                style: {
                    stroke: '#555',
                    strokeWidth: 2,
                    strokeDasharray: rel.relation_type === 'capital_affiliate' ? '5,5' : 'none',
                },
                markerEnd: {
                    type: MarkerType.ArrowClosed,
                    color: '#555',
                },
            }));
    }, [relations]);

    const createBusinessNodes = useCallback((focusCompanyId: number): Node[] => {
        const relatedIds = new Set([focusCompanyId]);

        relations.forEach(rel => {
            if (rel.relation_type.startsWith('business')) {
                if (rel.from_id === focusCompanyId) relatedIds.add(rel.to_id!);
                if (rel.to_id === focusCompanyId) relatedIds.add(rel.from_id!);
            }
        });

        // 親会社も追加
        relations.forEach(rel => {
            if (rel.relation_type.startsWith('capital') && rel.child_id && relatedIds.has(rel.child_id)) {
                if (rel.parent_id) relatedIds.add(rel.parent_id);
            }
        });

        // #970: 登録順の円形配置ではなく、起点企業からの関係性(BFS距離)に基づいて配置する
        const involvedCompanies = Array.from(relatedIds);
        const positions = layoutBusinessGraph(focusCompanyId, involvedCompanies, relations);

        return involvedCompanies.map((compId) => {
            const isFocusCompany = compId === focusCompanyId;
            const marketType = getMarketType(compId);
            const pos = positions.get(compId) ?? { x: 0, y: 0 };

            return {
                id: String(compId),
                type: 'default',
                position: { x: pos.x, y: pos.y },
                data: {
                    label: (
                        <Box sx={{ textAlign: 'center', p: 1 }}>
                            <Typography
                                variant="body2"
                                sx={{ fontWeight: isFocusCompany ? 'bold' : 'normal', mb: 0.5 }}
                            >
                                {getCompanyName(compId)}
                            </Typography>
                            <Chip
                                label={marketLabels[marketType]}
                                size="small"
                                sx={{
                                    bgcolor: marketColors[marketType],
                                    color: 'white',
                                    fontSize: '10px',
                                    height: '20px',
                                }}
                            />
                        </Box>
                    ),
                },
                style: {
                    background: isFocusCompany ? '#FFF3CD' : '#fff',
                    border: `3px solid ${isFocusCompany ? '#FFC107' : marketColors[marketType]}`,
                    borderRadius: '8px',
                    padding: '10px',
                    minWidth: '200px',
                    boxShadow: isFocusCompany ? '0 4px 12px rgba(255, 193, 7, 0.3)' : undefined,
                },
            };
        });
    }, [relations, getMarketType, getCompanyName]);

    const createBusinessEdges = useCallback((focusCompanyId: number): Edge[] => {
        const edges: Edge[] = [];
        const relatedIds = new Set([focusCompanyId]);

        relations.forEach(rel => {
            if (rel.relation_type.startsWith('business')) {
                if (rel.from_id === focusCompanyId) relatedIds.add(rel.to_id!);
                if (rel.to_id === focusCompanyId) relatedIds.add(rel.from_id!);
            }
        });

        // ビジネス関係のエッジ
        relations.forEach((rel, idx) => {
            if (rel.relation_type.startsWith('business') && rel.from_id && rel.to_id) {
                if (rel.from_id === focusCompanyId || rel.to_id === focusCompanyId ||
                    (relatedIds.has(rel.from_id) && relatedIds.has(rel.to_id))) {
                    edges.push({
                        id: `business-${idx}`,
                        source: String(rel.from_id),
                        target: String(rel.to_id),
                        type: 'custom',
                        label: formatRelationLabel(rel.description, rel.relation_type),
                        animated: true,
                        style: {
                            stroke: '#2196F3',
                            strokeWidth: 2,
                        },
                        markerEnd: {
                            type: MarkerType.ArrowClosed,
                            color: '#2196F3',
                        },
                    });
                }
            }
        });

        // 親会社との資本関係
        relations.forEach((rel, idx) => {
            if (rel.relation_type.startsWith('capital') && rel.parent_id && rel.child_id) {
                if (relatedIds.has(rel.child_id) && relatedIds.has(rel.parent_id)) {
                    edges.push({
                        id: `parent-${idx}`,
                        source: String(rel.parent_id),
                        target: String(rel.child_id),
                        type: 'custom',
                        animated: false,
                        style: {
                            stroke: '#999',
                            strokeWidth: 1,
                            strokeDasharray: '2,2',
                        },
                    });
                }
            }
        });

        return edges;
    }, [relations]);

    const { nodes, edges } = useMemo(() => {
        if (loading || relations.length === 0) {
            return { nodes: [], edges: [] };
        }

        if (diagramType === 'capital') {
            return {
                nodes: createCapitalNodes(companyId),
                edges: createCapitalEdges(companyId),
            };
        } else {
            return {
                nodes: createBusinessNodes(companyId),
                edges: createBusinessEdges(companyId),
            };
        }
    }, [diagramType, companyId, loading, relations, createCapitalNodes, createCapitalEdges, createBusinessNodes, createBusinessEdges]);

    const [flowNodes, setFlowNodes, onNodesChange] = useNodesState(nodes);
    const [flowEdges, setFlowEdges, onEdgesChange] = useEdgesState(edges);

    useMemo(() => {
        setFlowNodes(nodes);
        setFlowEdges(edges);
    }, [nodes, edges, setFlowNodes, setFlowEdges]);

    if (loading) {
        return (
            <Box sx={{ width: '100%', height: '500px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography>読み込み中...</Typography>
            </Box>
        );
    }

    if (loadError) {
        return (
            <Box sx={{ width: '100%', height: '500px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography color="error">{loadError}</Typography>
            </Box>
        );
    }

    if (nodes.length === 0) {
        return (
            <Box sx={{ width: '100%', height: '500px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography>関連企業データがありません</Typography>
            </Box>
        );
    }

    return (
        <Box sx={{ width: '100%', height: '500px' }}>
            <ReactFlow
                nodes={flowNodes}
                edges={flowEdges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                edgeTypes={edgeTypes}
                fitView
                minZoom={0.1}
                maxZoom={2}
            >
                <Background />
                <Controls />
                <MiniMap />
            </ReactFlow>
        </Box>
    );
}
