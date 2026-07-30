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

type DiagramType = 'capital' | 'business';

const CustomEdge = ({ id, sourceX, sourceY, targetX, targetY, style, markerEnd, label }: any) => {
    const edgePath = `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`;

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
                <text>
                    <textPath href={`#${id}`} startOffset="50%" textAnchor="middle" style={{ fontSize: '12px', fill: '#555' }}>
                        {label}
                    </textPath>
                </text>
            )}
        </>
    );
};

const edgeTypes: EdgeTypes = {
    custom: CustomEdge,
};

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

        const nodes: Node[] = [];
        const processedIds = new Set<number>();

        const addNodeWithChildren = (compId: number, level: number, xOffset: number): number => {
            if (processedIds.has(compId)) return xOffset;
            processedIds.add(compId);

            const children = relations.filter(rel =>
                rel.relation_type.startsWith('capital') &&
                rel.parent_id === compId &&
                relatedIds.has(rel.child_id!)
            );

            let currentX = xOffset;
            const childPositions: number[] = [];

            children.forEach((rel) => {
                if (rel.child_id) {
                    const childX = addNodeWithChildren(rel.child_id, level + 1, currentX);
                    childPositions.push((currentX + childX) / 2);
                    currentX = childX + 250;
                }
            });

            const nodeX = childPositions.length > 0
                ? (childPositions[0] + childPositions[childPositions.length - 1]) / 2
                : currentX;

            const isFocusCompany = compId === focusCompanyId;
            const marketType = getMarketType(compId);

            nodes.push({
                id: String(compId),
                type: 'default',
                position: { x: nodeX, y: level * 150 },
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
            });

            return currentX;
        };

        // トップレベルの親会社を見つける
        const parentCompanies = Array.from(relatedIds).filter(id => {
            return !relations.some(rel =>
                rel.relation_type.startsWith('capital') &&
                rel.child_id === id &&
                relatedIds.has(rel.parent_id!)
            );
        });

        let currentXOffset = 0;
        parentCompanies.forEach(parentId => {
            currentXOffset = addNodeWithChildren(parentId, 0, currentXOffset);
            currentXOffset += 300;
        });

        return nodes;
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

        const involvedCompanies = Array.from(relatedIds);
        const angle = (2 * Math.PI) / involvedCompanies.length;
        const radius = 250;

        return involvedCompanies.map((compId, idx) => {
            const isFocusCompany = compId === focusCompanyId;
            const marketType = getMarketType(compId);

            return {
                id: String(compId),
                type: 'default',
                position: {
                    x: 400 + radius * Math.cos(idx * angle),
                    y: 300 + radius * Math.sin(idx * angle),
                },
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
