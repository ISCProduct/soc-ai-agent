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
    type NodeMouseHandler,
} from 'reactflow';
import 'reactflow/dist/style.css';
import {
    Box,
    Typography,
    ToggleButtonGroup,
    ToggleButton,
    Chip,
    Autocomplete,
    TextField,
    FormControl,
    Button,
} from '@mui/material';
import {
    fetchCompanyRelations,
    fetchCompanyMarketInfo,
    marketColors,
    marketLabels,
    type CapitalRelation,
    type CompanyMarketInfo,
    type MarketType,
} from '@/lib/company-data';
import { formatRelationLabel } from '@/lib/relation-labels';
import { layoutBusinessGraph, layoutCapitalGraphFromEdges, computeRelationClusters, type RelationCluster } from '@/lib/relation-graph';
import { getCompanyIdFromNode, parseCompanyId } from '@/lib/correlation-diagram-navigation';
import CorrelationCompanyDetailPanel, {
    CORRELATION_DETAIL_PANEL_WIDTH,
} from '@/components/CorrelationCompanyDetailPanel';
import { edgeTypes } from '@/components/diagram/RelationEdge';
import ClusterExplorer from '@/components/diagram/ClusterExplorer';

type DiagramType = 'capital' | 'business';

interface CorrelationDiagramProps {
    initialCompanyId?: number | null
}

export default function CorrelationDiagram({ initialCompanyId = null }: CorrelationDiagramProps) {
    const [diagramType, setDiagramType] = useState<DiagramType>('capital');
    const [selectedCompanyId, setSelectedCompanyId] = useState<number | null>(initialCompanyId);
    const [detailCompanyId, setDetailCompanyId] = useState<number | null>(null);
    const [relations, setRelations] = useState<CapitalRelation[]>([]);
    const [marketInfo, setMarketInfo] = useState<CompanyMarketInfo[]>([]);
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [reloadToken, setReloadToken] = useState(0);

    useEffect(() => {
        async function loadData() {
            setLoading(true);
            setLoadError(null);
            try {
                const [relationsData, marketData] = await Promise.all([
                    fetchCompanyRelations(),
                    fetchCompanyMarketInfo(),
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
    }, [reloadToken]);

    // 企業選択の検索候補。#970フォローアップ: 以前は先頭100件のみ(登録順=ID順)に
    // 切り詰めていたため、それ以降に登録された企業が検索・選択できなかった。
    // Autocompleteは大量の選択肢でも検索(タイプ絞り込み)で実用的に扱えるため上限を設けない。
    const uniqueCompanies = useMemo(() => {
        const companyMap = new Map<number, string>();
        relations.forEach((rel) => {
            if (rel.parent) companyMap.set(rel.parent.id, rel.parent.name);
            if (rel.child) companyMap.set(rel.child.id, rel.child.name);
            if (rel.from) companyMap.set(rel.from.id, rel.from.name);
            if (rel.to) companyMap.set(rel.to.id, rel.to.name);
        });
        return Array.from(companyMap.entries()).sort((a, b) => a[1].localeCompare(b[1], 'ja'));
    }, [relations]);

    const getMarketType = useCallback((compId: number): MarketType => {
        const info = marketInfo.find((m) => m.company_id === compId);
        return info?.market_type || 'unlisted';
    }, [marketInfo]);

    const getCompanyName = useCallback((compId: number): string => {
        for (const rel of relations) {
            if (rel.parent?.id === compId) return rel.parent.name;
            if (rel.child?.id === compId) return rel.child.name;
            if (rel.from?.id === compId) return rel.from.name;
            if (rel.to?.id === compId) return rel.to.name;
        }
        return `企業 ${compId}`;
    }, [relations]);

    const createNodes = useCallback((focusCompanyId: number | null, type: DiagramType): Node[] => {
        // #970フォローアップ: 起点企業を指定しない「全体表示」は、ReactFlowで数百社を
        // まとめて描画すると本質的に読みにくい(ハブ企業由来の交差エッジ、fitViewの過剰縮小)ため、
        // ClusterExplorer(クラスタカード一覧)に置き換えた。ここでは起点企業がある場合のみ描画する。
        if (!focusCompanyId) {
            return [];
        }

        const relatedIds = new Set([focusCompanyId]);

        relations.forEach((rel) => {
            if (type === 'capital' && rel.relation_type.startsWith('capital')) {
                if (rel.parent_id === focusCompanyId || rel.child_id === focusCompanyId) {
                    if (rel.parent_id) relatedIds.add(rel.parent_id);
                    if (rel.child_id) relatedIds.add(rel.child_id);
                }
            } else if (type === 'business' && rel.relation_type.startsWith('business')) {
                if (rel.from_id === focusCompanyId) relatedIds.add(rel.to_id!);
                if (rel.to_id === focusCompanyId) relatedIds.add(rel.from_id!);
            }
        });

        const nodes: Node[] = [];
        const ids = Array.from(relatedIds);
        // #970: 登録順の円形配置ではなく、起点企業からの関係性(BFS距離)に基づいて配置する
        const positions =
            type === 'capital'
                ? layoutCapitalGraphFromEdges(focusCompanyId, ids, relations)
                : layoutBusinessGraph(focusCompanyId, ids, relations);

        ids.forEach((compId) => {
            const isFocusCompany = compId === focusCompanyId;
            const marketType = getMarketType(compId);
            const pos = positions.get(compId) ?? { x: 0, y: 0 };

            nodes.push({
                id: String(compId),
                type: 'default',
                position: { x: pos.x, y: pos.y },
                data: {
                    companyId: compId,
                    label: (
                        <Box sx={{ textAlign: 'center', p: 1.5, minWidth: '140px' }}>
                            <Typography
                                variant="body2"
                                sx={{
                                    fontWeight: isFocusCompany ? 700 : 500,
                                    fontSize: isFocusCompany ? '14px' : '13px',
                                    mb: 0.5,
                                    lineHeight: 1.3,
                                    wordBreak: 'break-word',
                                }}
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
                                    fontWeight: 500,
                                }}
                            />
                        </Box>
                    ),
                },
                style: {
                    background: isFocusCompany ? '#FFF3CD' : '#fff',
                    border: `${isFocusCompany ? 3 : 2}px solid ${
                        isFocusCompany ? '#FFA726' : marketColors[marketType]
                    }`,
                    borderRadius: '8px',
                    padding: isFocusCompany ? '10px' : '8px',
                    minWidth: isFocusCompany ? '180px' : '160px',
                    boxShadow: isFocusCompany
                        ? '0 4px 12px rgba(255,167,38,0.3)'
                        : '0 2px 8px rgba(0,0,0,0.1)',
                    cursor: 'pointer',
                },
            });
        });

        return nodes;
    }, [relations, getMarketType, getCompanyName]);

    const createEdges = useCallback((focusCompanyId: number | null, type: DiagramType): Edge[] => {
        if (!focusCompanyId) {
            return [];
        }
        const edges: Edge[] = [];

        relations.forEach((rel, idx) => {
            if (type === 'capital' && rel.relation_type.startsWith('capital') && rel.parent_id && rel.child_id) {
                edges.push({
                    id: `capital-${idx}`,
                    source: String(rel.parent_id),
                    target: String(rel.child_id),
                    type: 'custom',
                    label: rel.ratio ? `${rel.ratio.toFixed(0)}%` : '',
                    style: {
                        stroke: '#555',
                        strokeWidth: 2,
                        strokeDasharray: rel.relation_type === 'capital_affiliate' ? '5,5' : 'none',
                    },
                    markerEnd: {
                        type: MarkerType.ArrowClosed,
                        color: '#555',
                    },
                });
            } else if (type === 'business' && rel.relation_type.startsWith('business') && rel.from_id && rel.to_id) {
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
        });

        return edges;
    }, [relations]);

    const { nodes, edges } = useMemo(() => {
        if (loading || relations.length === 0) {
            return { nodes: [], edges: [] };
        }

        return {
            nodes: createNodes(selectedCompanyId, diagramType),
            edges: createEdges(selectedCompanyId, diagramType),
        };
    }, [diagramType, selectedCompanyId, loading, relations, createNodes, createEdges]);

    const [flowNodes, setFlowNodes, onNodesChange] = useNodesState(nodes);
    const [flowEdges, setFlowEdges, onEdgesChange] = useEdgesState(edges);

    useEffect(() => {
        setFlowNodes(nodes);
        setFlowEdges(edges);
    }, [nodes, edges, setFlowNodes, setFlowEdges]);

    // 詳細選択のハイライトだけ更新し、ドラッグ済みの位置は保持する
    useEffect(() => {
        setFlowNodes((current) =>
            current.map((node) => {
                const id = getCompanyIdFromNode(node);
                const isDetail = detailCompanyId !== null && id === detailCompanyId;
                const isFocus = selectedCompanyId !== null && id === selectedCompanyId;
                const marketType = id !== null ? getMarketType(id) : ('unlisted' as MarketType);
                const prev = node.style ?? {};
                return {
                    ...node,
                    style: {
                        ...prev,
                        background: isFocus ? '#FFF3CD' : isDetail ? '#E3F2FD' : '#fff',
                        border: `${isFocus || isDetail ? 3 : 2}px solid ${
                            isFocus ? '#FFA726' : isDetail ? '#1976D2' : marketColors[marketType]
                        }`,
                    },
                };
            })
        );
    }, [detailCompanyId, selectedCompanyId, getMarketType, setFlowNodes]);

    const handleNodeClick: NodeMouseHandler = useCallback((_event, node) => {
        const companyId = getCompanyIdFromNode(node);
        if (companyId === null) return;
        setDetailCompanyId(companyId);
    }, []);

    const handleOpenSelectedCompanyDetail = useCallback(() => {
        const companyId = parseCompanyId(selectedCompanyId);
        if (companyId === null) return;
        setDetailCompanyId(companyId);
    }, [selectedCompanyId]);

    const handleCloseDetailPanel = useCallback(() => {
        setDetailCompanyId(null);
    }, []);

    const canOpenSelectedCompanyDetail = parseCompanyId(selectedCompanyId) !== null;

    if (loading) {
        return (
            <Box sx={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography>読み込み中...</Typography>
            </Box>
        );
    }

    if (loadError) {
        return (
            <Box sx={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', px: 2 }}>
                <Typography color="error">{loadError}</Typography>
            </Box>
        );
    }

    if (relations.length === 0) {
        return (
            <Box sx={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', px: 2 }}>
                <Typography color="text.secondary" align="center">
                    企業関係データがありません。管理画面から企業情報の取得・関連付けを行ってください。
                </Typography>
            </Box>
        );
    }

    const showDetailPanel = detailCompanyId !== null;


    return (
        <Box
            sx={{
                width: '100%',
                height: '100%',
                display: 'flex',
                flexDirection: 'column',
                overflow: 'hidden',
                bgcolor: '#fafafa',
            }}
        >
            <Box sx={{ px: 2, py: 1.5, bgcolor: '#f5f5f5', borderBottom: '1px solid #ddd', flexShrink: 0 }}>
                <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'center', flexWrap: 'wrap' }}>
                    <ToggleButtonGroup
                        value={diagramType}
                        exclusive
                        onChange={(_e, value) => value && setDiagramType(value)}
                        size="small"
                    >
                        <ToggleButton value="capital">資本関連図</ToggleButton>
                        <ToggleButton value="business">ビジネス関連図</ToggleButton>
                    </ToggleButtonGroup>

                    <FormControl size="small" sx={{ minWidth: { xs: '100%', sm: 260 }, maxWidth: 360 }}>
                        <Autocomplete
                            size="small"
                            options={uniqueCompanies}
                            getOptionLabel={([, name]) => name}
                            isOptionEqualToValue={([idA], [idB]) => idA === idB}
                            value={uniqueCompanies.find(([id]) => id === selectedCompanyId) ?? null}
                            onChange={(_e, option) => setSelectedCompanyId(option ? option[0] : null)}
                            renderInput={(params) => (
                                <TextField {...params} label="企業を検索" placeholder="全体表示(クラスタ一覧)" />
                            )}
                        />
                    </FormControl>

                    <Button
                        variant="outlined"
                        size="small"
                        disabled={!canOpenSelectedCompanyDetail}
                        onClick={handleOpenSelectedCompanyDetail}
                    >
                        選択企業の情報
                    </Button>

                    <Typography variant="caption" color="text.secondary" sx={{ flexBasis: { xs: '100%', md: 'auto' } }}>
                        企業ノードをクリックすると右側に情報を表示します
                    </Typography>

                    <Box sx={{ display: 'flex', gap: 1.5, ml: { md: 'auto' }, flexWrap: 'wrap' }}>
                        {Object.entries(marketLabels).map(([key, label]) => (
                            <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                                <Box sx={{ width: 14, height: 14, bgcolor: marketColors[key as MarketType], borderRadius: '50%' }} />
                                <Typography variant="caption">{label}</Typography>
                            </Box>
                        ))}
                    </Box>
                </Box>

                {diagramType === 'capital' && (
                    <Box sx={{ mt: 1, display: 'flex', gap: 3, fontSize: '12px', flexWrap: 'wrap' }}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                            <Box sx={{ width: 40, height: 2, bgcolor: '#555' }} />
                            <span>子会社（実線）</span>
                        </Box>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                            <Box sx={{ width: 40, height: 2, borderTop: '2px dashed #555' }} />
                            <span>関連会社（破線）</span>
                        </Box>
                    </Box>
                )}
            </Box>

            <Box
                sx={{
                    flex: 1,
                    minHeight: 0,
                    display: 'flex',
                    flexDirection: { xs: 'column', md: 'row' },
                    overflow: 'hidden',
                }}
            >
                <Box
                    sx={{
                        flex: 1,
                        minWidth: 0,
                        minHeight: { xs: showDetailPanel ? 280 : 0, md: 0 },
                        height: { xs: showDetailPanel ? '55%' : '100%', md: '100%' },
                        position: 'relative',
                        bgcolor: '#fff',
                    }}
                >
                    {selectedCompanyId === null ? (
                        <ClusterExplorer
                            relations={relations}
                            diagramType={diagramType}
                            getCompanyName={getCompanyName}
                            getMarketType={getMarketType}
                            onSelectCompany={setSelectedCompanyId}
                        />
                    ) : (
                        <ReactFlow
                            nodes={flowNodes}
                            edges={flowEdges}
                            onNodesChange={onNodesChange}
                            onEdgesChange={onEdgesChange}
                            onNodeClick={handleNodeClick}
                            edgeTypes={edgeTypes}
                            fitView
                            minZoom={0.05}
                            maxZoom={3}
                            defaultViewport={{ x: 0, y: 0, zoom: 0.8 }}
                            attributionPosition="bottom-right"
                            nodesDraggable={true}
                            nodesConnectable={false}
                            elementsSelectable={true}
                            style={{ width: '100%', height: '100%' }}
                        >
                            <Background color="#aaa" gap={16} />
                            <Controls
                                showZoom={true}
                                showFitView={true}
                                showInteractive={true}
                                position="top-right"
                            />
                            <MiniMap
                                nodeColor={(node) => {
                                    const border = node.style?.border as string;
                                    if (border?.includes('#FFA726')) return '#FFA726';
                                    if (border?.includes('#1976D2')) return '#1976D2';
                                    return '#2196F3';
                                }}
                                maskColor="rgba(0, 0, 0, 0.1)"
                                position="bottom-left"
                            />
                        </ReactFlow>
                    )}
                </Box>

                {showDetailPanel && (
                    <Box
                        sx={{
                            width: { xs: '100%', md: CORRELATION_DETAIL_PANEL_WIDTH },
                            height: { xs: '45%', md: '100%' },
                            flexShrink: 0,
                            minHeight: { xs: 240, md: 0 },
                        }}
                    >
                        <CorrelationCompanyDetailPanel
                            companyId={detailCompanyId}
                            marketType={getMarketType(detailCompanyId)}
                            onClose={handleCloseDetailPanel}
                        />
                    </Box>
                )}
            </Box>
        </Box>
    );
}
