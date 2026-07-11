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
    EdgeTypes,
} from 'reactflow';
import 'reactflow/dist/style.css';
import { Card, CardContent } from '@/components/ui/card';
import { Box, Typography, ToggleButtonGroup, ToggleButton, Chip, Select, MenuItem, FormControl, InputLabel } from '@mui/material';
import {
    fetchCompanyRelations,
    fetchCompanyMarketInfo,
    marketColors,
    marketLabels,
    type CapitalRelation,
    type CompanyMarketInfo,
    type MarketType,
} from '@/lib/company-data';

type DiagramType = 'capital' | 'business';

const CustomEdge = ({ id, sourceX, sourceY, targetX, targetY, style, markerEnd, label }: any) => {
    const edgePath = `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`;

    // ラベルの位置を計算（中点）
    const labelX = (sourceX + targetX) / 2;
    const labelY = (sourceY + targetY) / 2;

    // エッジの角度を計算
    const angle = Math.atan2(targetY - sourceY, targetX - sourceX) * (180 / Math.PI);

    // テキストが逆さまにならないように調整（-90度〜90度の範囲に収める）
    const adjustedAngle = angle > 90 || angle < -90 ? angle + 180 : angle;

    return (
        <>
            <path
                id={id}
                style={style}
                className="react-flow__edge-path"
                d={edgePath}
                markerEnd={markerEnd}
            />
            {label && (
                <text
                    x={labelX}
                    y={labelY}
                    style={{
                        fontSize: '13px',
                        fill: '#333',
                        fontWeight: 600,
                        pointerEvents: 'none',
                    }}
                    textAnchor="middle"
                    dominantBaseline="middle"
                    transform={`rotate(${adjustedAngle}, ${labelX}, ${labelY})`}
                >
                    {/* 白い縁取り（背景） */}
                    <tspan
                        x={labelX}
                        dy="0"
                        style={{
                            fill: 'none',
                            stroke: '#fff',
                            strokeWidth: 4,
                            strokeLinejoin: 'round',
                            paintOrder: 'stroke',
                        }}
                    >
                        {label}
                    </tspan>
                    {/* メインテキスト */}
                    <tspan
                        x={labelX}
                        dy="0"
                        style={{
                            fill: '#333',
                        }}
                    >
                        {label}
                    </tspan>
                </text>
            )}
        </>
    );
};

const edgeTypes: EdgeTypes = {
    custom: CustomEdge,
};

interface CorrelationDiagramProps {
    initialCompanyId?: number | null
}

export default function CorrelationDiagram({ initialCompanyId = null }: CorrelationDiagramProps) {
    const [diagramType, setDiagramType] = useState<DiagramType>('capital');
    const [selectedCompanyId, setSelectedCompanyId] = useState<number | null>(initialCompanyId);
    const [relations, setRelations] = useState<CapitalRelation[]>([]);
    const [marketInfo, setMarketInfo] = useState<CompanyMarketInfo[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        async function loadData() {
            setLoading(true);
            setError(null);
            try {
                const [relationsData, marketData] = await Promise.all([
                    fetchCompanyRelations(),
                    fetchCompanyMarketInfo()
                ]);
                setRelations(relationsData);
                setMarketInfo(marketData);
            } catch {
                setError('関係データの取得に失敗しました');
                setRelations([]);
                setMarketInfo([]);
            } finally {
                setLoading(false);
            }
        }
        void loadData();
    }, []);

    const uniqueCompanies = useMemo(() => {
        const companyMap = new Map();
        relations.forEach(rel => {
            if (rel.parent) companyMap.set(rel.parent.id, rel.parent.name);
            if (rel.child) companyMap.set(rel.child.id, rel.child.name);
            if (rel.from) companyMap.set(rel.from.id, rel.from.name);
            if (rel.to) companyMap.set(rel.to.id, rel.to.name);
        });
        return Array.from(companyMap.entries()).sort((a, b) => a[0] - b[0]);
    }, [relations]);

    const getMarketType = useCallback((compId: number): MarketType => {
        const info = marketInfo.find(m => m.company_id === compId);
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
        if (!focusCompanyId) {
            const limitedRelations = relations.slice(0, 100);
            const companyIds = new Set<number>();
            limitedRelations.forEach(rel => {
                if (type === 'capital') {
                    if (rel.parent_id) companyIds.add(rel.parent_id);
                    if (rel.child_id) companyIds.add(rel.child_id);
                } else {
                    if (rel.from_id) companyIds.add(rel.from_id);
                    if (rel.to_id) companyIds.add(rel.to_id);
                }
            });

            const nodes: Node[] = [];
            const ids = Array.from(companyIds);
            const cols = Math.ceil(Math.sqrt(ids.length));

            ids.forEach((id, idx) => {
                const row = Math.floor(idx / cols);
                const col = idx % cols;
                const marketType = getMarketType(id);

                nodes.push({
                    id: String(id),
                    type: 'default',
                    position: { x: col * 300, y: row * 180 },
                    data: {
                        label: (
                            <Box sx={{ textAlign: 'center', p: 1, minWidth: '140px' }}>
                                <Typography
                                    variant="body2"
                                    sx={{
                                        fontSize: '13px',
                                        fontWeight: 500,
                                        lineHeight: 1.3,
                                        mb: 0.5,
                                        wordBreak: 'break-word',
                                    }}
                                >
                                    {getCompanyName(id).length > 20
                                        ? getCompanyName(id).substring(0, 20) + '...'
                                        : getCompanyName(id)
                                    }
                                </Typography>
                                <Chip
                                    label={marketLabels[marketType]}
                                    size="small"
                                    sx={{
                                        bgcolor: marketColors[marketType],
                                        color: 'white',
                                        fontSize: '10px',
                                        height: '18px',
                                        fontWeight: 500,
                                    }}
                                />
                            </Box>
                        ),
                    },
                    style: {
                        background: '#fff',
                        border: `2px solid ${marketColors[marketType]}`,
                        borderRadius: '8px',
                        padding: '8px',
                        minWidth: '160px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                    },
                });
            });

            return nodes;
        }

        const relatedIds = new Set([focusCompanyId]);

        relations.forEach(rel => {
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
        const angle = (2 * Math.PI) / ids.length;
        const radius = 250;

        ids.forEach((compId, idx) => {
            const isFocusCompany = compId === focusCompanyId;
            const marketType = getMarketType(compId);

            nodes.push({
                id: String(compId),
                type: 'default',
                position: {
                    x: 500 + radius * Math.cos(idx * angle),
                    y: 400 + radius * Math.sin(idx * angle),
                },
                data: {
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
                    border: `${isFocusCompany ? 3 : 2}px solid ${isFocusCompany ? '#FFA726' : marketColors[marketType]}`,
                    borderRadius: '8px',
                    padding: isFocusCompany ? '10px' : '8px',
                    minWidth: isFocusCompany ? '180px' : '160px',
                    boxShadow: isFocusCompany ? '0 4px 12px rgba(255,167,38,0.3)' : '0 2px 8px rgba(0,0,0,0.1)',
                },
            });
        });

        return nodes;
    }, [relations, getMarketType, getCompanyName]);

    const createEdges = useCallback((focusCompanyId: number | null, type: DiagramType): Edge[] => {
        const edges: Edge[] = [];
        let relevantRelations = relations;

        if (!focusCompanyId) {
            relevantRelations = relations.slice(0, 100);
        }

        relevantRelations.forEach((rel, idx) => {
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
                    label: rel.description || rel.relation_type,
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

    useMemo(() => {
        setFlowNodes(nodes);
        setFlowEdges(edges);
    }, [nodes, edges, setFlowNodes, setFlowEdges]);

    if (loading) {
        return (
            <Box sx={{ width: '100%', height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography>読み込み中...</Typography>
            </Box>
        );
    }

    if (error) {
        return (
            <Box sx={{ width: '100%', height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Typography color="error">{error}</Typography>
            </Box>
        );
    }

    return (
        <Box sx={{ width: '100%', height: '100vh', display: 'flex', flexDirection: 'column' }}>
            <Box sx={{ p: 2, bgcolor: '#f5f5f5', borderBottom: '1px solid #ddd' }}>
                <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', flexWrap: 'wrap' }}>
                    <ToggleButtonGroup
                        value={diagramType}
                        exclusive
                        onChange={(e, value) => value && setDiagramType(value)}
                        size="small"
                    >
                        <ToggleButton value="capital">資本関連図</ToggleButton>
                        <ToggleButton value="business">ビジネス関連図</ToggleButton>
                    </ToggleButtonGroup>

                    <FormControl size="small" sx={{ minWidth: 300 }}>
                        <InputLabel>企業選択</InputLabel>
                        <Select
                            value={selectedCompanyId || ''}
                            onChange={(e) => setSelectedCompanyId(e.target.value as number || null)}
                            label="企業選択"
                        >
                            <MenuItem value="">全体表示</MenuItem>
                            {uniqueCompanies.slice(0, 100).map(([id, name]) => (
                                <MenuItem key={id} value={id}>{name}</MenuItem>
                            ))}
                        </Select>
                    </FormControl>

                    <Box sx={{ display: 'flex', gap: 2, ml: 'auto' }}>
                        {Object.entries(marketLabels).map(([key, label]) => (
                            <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                                <Box sx={{ width: 16, height: 16, bgcolor: marketColors[key as MarketType], borderRadius: '50%' }} />
                                <Typography variant="caption">{label}</Typography>
                            </Box>
                        ))}
                    </Box>
                </Box>

                {diagramType === 'capital' && (
                    <Box sx={{ mt: 1, display: 'flex', gap: 3, fontSize: '12px' }}>
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

            <Card style={{ height: 'calc(100% - 100px)', flex: 1 }}>
                <CardContent style={{ height: '100%', padding: 0 }}>
                    <ReactFlow
                        nodes={flowNodes}
                        edges={flowEdges}
                        onNodesChange={onNodesChange}
                        onEdgesChange={onEdgesChange}
                        edgeTypes={edgeTypes}
                        fitView
                        minZoom={0.05}
                        maxZoom={3}
                        defaultViewport={{ x: 0, y: 0, zoom: 0.8 }}
                        attributionPosition="bottom-right"
                        nodesDraggable={true}
                        nodesConnectable={false}
                        elementsSelectable={true}
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
                                return '#2196F3';
                            }}
                            maskColor="rgba(0, 0, 0, 0.1)"
                            position="bottom-left"
                        />
                    </ReactFlow>
                </CardContent>
            </Card>
        </Box>
    );
}
