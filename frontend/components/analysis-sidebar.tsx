'use client'
import React, {useState, useEffect} from 'react'
import styles from './analysis-sidebar.module.css'
import
{
    Box,
    Drawer,
    List,
    ListItem,
    ListItemButton,
    ListItemIcon,
    ListItemText,
    Typography,
    LinearProgress,
    Divider,
    Chip,
    Avatar,
    IconButton,
    useTheme,
    useMediaQuery,
}
    from '@mui/material'
import {
    BorderAll,
    CheckCircle,
    RadioButtonUnchecked,
    Work,
    Psychology,
    TrendingUp,
    Speed,
    EmojiEvents,
    Logout,
    History,
    ManageAccounts,
    Description,
    AdminPanelSettings,
    RecordVoiceOver,
    EditNote,
    CalendarMonth,
} from '@mui/icons-material'
import {authService, User} from '@/lib/auth'
import {useRouter} from 'next/navigation'

const DRAWER_WIDTH = 280

interface AnalysisStep {
    id: string
    label: string
    icon: React.ReactNode
    completed: boolean
    progress?: number
}

interface PhaseProgress {
    phase_name: string
    display_name: string
    questions_asked: number
    valid_answers: number
    completion_score: number
    is_completed: boolean
    min_questions: number
    max_questions: number
}

interface AnalysisSidebarProps {
    user: User
    onLogout: () => void
    mobileOpen?: boolean
    onMobileClose?: () => void
}

export function AnalysisSidebar({user, onLogout, mobileOpen = false, onMobileClose}: AnalysisSidebarProps) {
    const [messageCount, setMessageCount] = useState(0)
    const [questionCount, setQuestionCount] = useState(0)
    const [totalQuestions, setTotalQuestions] = useState(15)
    const [phases, setPhases] = useState<PhaseProgress[] | null>(null)
    const [isAdmin, setIsAdmin] = useState(!!user.is_admin)
    const router = useRouter()
    const theme = useTheme()
    const isMobile = useMediaQuery(theme.breakpoints.down('md'))

    useEffect(() => {
        setIsAdmin(!!user.is_admin)
        const refreshAdminFlag = async () => {
            if (!user?.user_id || user.is_admin) return
            try {
                const fresh = await authService.getUser()
                if (fresh?.is_admin) {
                    setIsAdmin(true)
                    authService.saveAuth({ ...fresh, user_id: fresh.user_id, is_guest: fresh.is_guest } as any)
                }
            } catch {
                // ignore
            }
        }
        refreshAdminFlag()

        const handleChatProgress = (event: CustomEvent) => {
            setMessageCount(event.detail.messageCount || 0)
            setQuestionCount(event.detail.questionCount || 0)
            setTotalQuestions(event.detail.totalQuestions || 15)
            if (event.detail.phases) {
                setPhases(event.detail.phases as PhaseProgress[])
            }
        }

        window.addEventListener('chatProgress', handleChatProgress as EventListener)
        return () => {
            window.removeEventListener('chatProgress', handleChatProgress as EventListener)
        }
    }, [user])

    const phaseProgressFor = (phaseName: string) => {
        if (!phases) return null
        return phases.find(p => p.phase_name === phaseName) || null
    }
    const getPhasePercent = (phaseName: string, fallback: number) => {
        const phase = phaseProgressFor(phaseName)
        if (!phase) return fallback
        const required = phase.max_questions > 0 ? phase.max_questions : phase.min_questions
        if (required > 0) {
            return Math.min(100, Math.max(0, Math.floor((phase.valid_answers / required) * 100)))
        }
        if (phase.questions_asked <= 0) return 0
        return Math.min(100, Math.floor((phase.valid_answers / phase.questions_asked) * 100))
    }
    const getPhaseStatus = (phaseName: string, defaultLabel: string) => {
        const phase = phaseProgressFor(phaseName)
        if (!phase) return defaultLabel
        if (phase.is_completed) return defaultLabel.replace('進行中', '完了').replace('待機中', '完了')
        if (phase.questions_asked > 0) return defaultLabel.replace('待機中', '進行中')
        return defaultLabel
    }

    const expectedTotalQuestions = (() => {
        if (!phases || phases.length === 0) return totalQuestions
        return phases.reduce((sum, phase) => {
            const required = phase.max_questions > 0 ? phase.max_questions : phase.min_questions
            return sum + (required > 0 ? required : 0)
        }, 0)
    })()

    const fallbackOverall = (() => {
        if (!phases || phases.length === 0) {
            return expectedTotalQuestions > 0
                ? Math.min(100, Math.floor((questionCount / expectedTotalQuestions) * 100))
                : 0
        }
        let valid = 0
        let asked = 0
        for (const phase of phases) {
            asked += phase.questions_asked
            valid += phase.valid_answers
        }
        if (asked <= 0) return 0
        return Math.min(100, Math.floor((valid / asked) * 100))
    })()
    const phasePercents = {
        job: getPhasePercent('job_analysis', fallbackOverall),
        interest: getPhasePercent('interest_analysis', fallbackOverall),
        aptitude: getPhasePercent('aptitude_analysis', fallbackOverall),
        future: getPhasePercent('future_analysis', fallbackOverall),
    }
    const progress = {
        overall: phases ? Math.floor((phasePercents.job + phasePercents.interest + phasePercents.aptitude + phasePercents.future) / 4) : fallbackOverall,
        ...phasePercents,
    }

    const analysisSteps: AnalysisStep[] = [
        {
            id: 'job',
            label: getPhaseStatus('job_analysis', progress.job === 100 ? '職種分析完了' : '職種分析進行中'),
            icon: <Work/>,
            completed: getPhasePercent('job_analysis', progress.job) === 100,
            progress: getPhasePercent('job_analysis', progress.job) < 100 ? getPhasePercent('job_analysis', progress.job) : undefined,
        },
        {
            id: 'interest',
            label: getPhaseStatus('interest_analysis', progress.interest === 100 ? '興味分析完了' : progress.interest > 0 ? '興味分析進行中' : '興味分析待機中'),
            icon: <Psychology/>,
            completed: getPhasePercent('interest_analysis', progress.interest) === 100,
            progress: getPhasePercent('interest_analysis', progress.interest) > 0 && getPhasePercent('interest_analysis', progress.interest) < 100 ? getPhasePercent('interest_analysis', progress.interest) : undefined,
        },
        {
            id: 'aptitude',
            label: getPhaseStatus('aptitude_analysis', progress.aptitude === 100 ? '適性分析完了' : progress.aptitude > 0 ? '適性分析進行中' : '適性分析待機中'),
            icon: <TrendingUp/>,
            completed: getPhasePercent('aptitude_analysis', progress.aptitude) === 100,
            progress: getPhasePercent('aptitude_analysis', progress.aptitude) > 0 && getPhasePercent('aptitude_analysis', progress.aptitude) < 100 ? getPhasePercent('aptitude_analysis', progress.aptitude) : undefined,
        },
        {
            id: 'future',
            label: getPhaseStatus('future_analysis', progress.future === 100 ? '将来分析完了' : progress.future > 0 ? '将来分析進行中' : '将来分析待機中'),
            icon: <EmojiEvents/>,
            completed: getPhasePercent('future_analysis', progress.future) === 100,
            progress: getPhasePercent('future_analysis', progress.future) > 0 && getPhasePercent('future_analysis', progress.future) < 100 ? getPhasePercent('future_analysis', progress.future) : undefined,
        },
    ]

    const drawerSx = {
        width: DRAWER_WIDTH,
        flexShrink: 0,
        '& .MuiDrawer-paper': {
            width: DRAWER_WIDTH,
            boxSizing: 'border-box',
            backgroundColor: '#f7f7f8',
            borderRight: '1px solid #e0e0e0',
            overflowY: 'auto',
        },
    }

    const drawerContent = (
            <Box sx={{p: 2}}>
                <Box sx={{display: 'flex', alignItems: 'center', mb: 2, gap: 1}}>
                    <Avatar sx={{bgcolor: user.is_guest ? 'grey.500' : 'primary.main'}}>
                        {user.name.charAt(0).toUpperCase()}
                    </Avatar>
                    <Box sx={{flex: 1, minWidth: 0}}>
                        <Typography variant="subtitle2" noWrap sx={{fontWeight: 600}}>
                            {user.name}
                        </Typography>
                        {user.is_guest && (
                            <Chip label="ゲスト" size="small" sx={{height: 18, fontSize: '0.7rem'}}/>
                        )}
                        {user.oauth_provider && (
                            <Chip
                                label={user.oauth_provider}
                                size="small"
                                sx={{height: 18, fontSize: '0.7rem', textTransform: 'capitalize'}}
                            />
                        )}
                    </Box>
                    <IconButton size="small" onClick={onLogout} title="ログアウト">
                        <Logout fontSize="small"/>
                    </IconButton>
                </Box>

                <Divider sx={{mb: 2}}/>

                {/* 今日やること（推奨フロー） */}
                {progress.overall < 100 && (
                    <Box sx={{ mb: 2, p: 1.5, bgcolor: '#fff8f5', borderRadius: 1, border: '1px solid #ffcc99' }}>
                        <Typography variant="caption" sx={{ fontWeight: 700, color: '#ec5b13', display: 'block', mb: 0.5 }}>
                            今日やること
                        </Typography>
                        <Typography variant="body2" sx={{ fontSize: '0.8rem', color: '#555' }}>
                            {progress.overall === 0
                                ? '① 自己分析チャットを始めましょう'
                                : progress.overall < 80
                                    ? '① チャットを続けて分析を完成させましょう'
                                    : '② マッチング結果を確認しましょう'}
                        </Typography>
                    </Box>
                )}

                <Typography variant="h6" sx={{fontWeight: 600, mb: 1}}>
                    AI分析進捗
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
                    質問: {questionCount}/{expectedTotalQuestions} 完了 (想定{expectedTotalQuestions}問・{progress.overall}%)
                </Typography>

                <List sx={{p: 0}}>
                    {analysisSteps.map((step, index) => (
                        <React.Fragment key={step.id}>
                            <ListItem
                                sx={{
                                    borderRadius: 1,
                                    mb: 0.5,
                                    backgroundColor: step.completed ? '#e8f5e9' : 'transparent',
                                    '&:hover': {
                                        backgroundColor: step.completed ? '#e8f5e9' : '#f0f0f0',
                                    },
                                }}
                            >
                                <ListItemIcon sx={{minWidth: 36}}>
                                    {step.completed ? (
                                        <CheckCircle color="success"/>
                                    ) : (
                                        <RadioButtonUnchecked color="action"/>
                                    )}
                                </ListItemIcon>
                                <ListItemText
                                    primary={step.label}
                                    primaryTypographyProps={{
                                        fontSize: '0.875rem',
                                        fontWeight: step.completed ? 500 : 400,
                                    }}
                                />
                            </ListItem>
                            {step.progress !== undefined && (
                                <Box sx={{px: 2, pb: 1}}>
                                    <LinearProgress
                                        variant="determinate"
                                        value={step.progress}
                                        sx={{height: 6, borderRadius: 3}}
                                    />
                                    <Typography
                                        variant="caption"
                                        color="text.secondary"
                                        sx={{mt: 0.5, display: 'block'}}
                                    >
                                        {step.progress}% 完了
                                    </Typography>
                                </Box>
                            )}
                        </React.Fragment>
                    ))}
                </List>

                <Divider sx={{my: 2}}/>

                <Box>
                    <Typography variant="subtitle2" sx={{mb: 1, fontWeight: 600}}>
                        IT業界キャリアエージェント
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{mb: 1}}>
                        AIが質問を動的に生成し、あなたの適性を分析
                    </Typography>
                    <Chip
                        label="AI分析中"
                        color="primary"
                        size="small"
                        sx={{fontSize: '0.75rem'}}
                    />
                </Box>


                <Divider sx={{my: 2}}/>
                <ListItem disablePadding>
                    <ListItemButton
                        onClick={() => router.push('/Correlation-diagram')}
                        sx={{
                            borderRadius: 1,
                        }}
                    >
                        <ListItemIcon sx={{minWidth: 36}}>
                            <BorderAll color="primary"/>
                        </ListItemIcon>
                        <ListItemText
                            primary="企業相関図"
                            primaryTypographyProps={{
                                fontSize: '0.875rem',
                                fontWeight: 500,
                            }}
                        />
                    </ListItemButton>
                </ListItem>

                <Divider sx={{my: 2}}/>

                <ListItem disablePadding>
                    <ListItemButton
                        onClick={() => router.push('/chat-history')}
                        sx={{
                            borderRadius: 1,
                        }}
                    >
                        <ListItemIcon sx={{minWidth: 36}}>
                            <History color="primary"/>
                        </ListItemIcon>
                        <ListItemText
                            primary="チャット履歴"
                            primaryTypographyProps={{
                                fontSize: '0.875rem',
                                fontWeight: 500,
                            }}
                        />
                    </ListItemButton>
                </ListItem>

                <ListItem disablePadding>
                    <ListItemButton
                        onClick={() => router.push('/resume')}
                        sx={{
                            borderRadius: 1,
                        }}
                    >
                        <ListItemIcon sx={{minWidth: 36}}>
                            <Description color="primary"/>
                        </ListItemIcon>
                        <ListItemText
                            primary="履歴書レビュー"
                            primaryTypographyProps={{
                                fontSize: '0.875rem',
                                fontWeight: 500,
                            }}
                        />
                    </ListItemButton>
                </ListItem>

                <ListItem disablePadding>
                    <ListItemButton
                        onClick={() => router.push('/interview')}
                        sx={{
                            borderRadius: 1,
                        }}
                    >
                        <ListItemIcon sx={{minWidth: 36}}>
                            <RecordVoiceOver color="primary"/>
                        </ListItemIcon>
                        <ListItemText
                            primary="面接練習"
                            primaryTypographyProps={{
                                fontSize: '0.875rem',
                                fontWeight: 500,
                            }}
                        />
                    </ListItemButton>
                </ListItem>

                <ListItem disablePadding>
                    <ListItemButton
                        onClick={() => router.push('/es-rewrite')}
                        sx={{
                            borderRadius: 1,
                        }}
                    >
                        <ListItemIcon sx={{minWidth: 36}}>
                            <EditNote color="primary"/>
                        </ListItemIcon>
                        <ListItemText
                            primary="ESリライト・添削"
                            primaryTypographyProps={{
                                fontSize: '0.875rem',
                                fontWeight: 500,
                            }}
                        />
                    </ListItemButton>
                </ListItem>

                <ListItem disablePadding>
                    <ListItemButton
                        onClick={() => router.push('/schedule')}
                        sx={{
                            borderRadius: 1,
                        }}
                    >
                        <ListItemIcon sx={{minWidth: 36}}>
                            <CalendarMonth color="primary"/>
                        </ListItemIcon>
                        <ListItemText
                            primary="選考スケジュール"
                            primaryTypographyProps={{
                                fontSize: '0.875rem',
                                fontWeight: 500,
                            }}
                        />
                    </ListItemButton>
                </ListItem>

                <ListItem disablePadding>
                    <ListItemButton
                        onClick={() => router.push('/onboarding')}
                        sx={{
                            borderRadius: 1,
                        }}
                    >
                        <ListItemIcon sx={{minWidth: 36}}>
                            <ManageAccounts color="primary"/>
                        </ListItemIcon>
                        <ListItemText
                            primary="プロフィール設定"
                            primaryTypographyProps={{
                                fontSize: '0.875rem',
                                fontWeight: 500,
                            }}
                        />
                    </ListItemButton>
                </ListItem>

                {isAdmin && (
                    <ListItem disablePadding>
                        <ListItemButton
                            onClick={() => router.push('/admin')}
                            sx={{
                                borderRadius: 1,
                            }}
                        >
                            <ListItemIcon sx={{minWidth: 36}}>
                                <AdminPanelSettings color="primary"/>
                            </ListItemIcon>
                            <ListItemText
                                primary="管理者機能"
                                primaryTypographyProps={{
                                    fontSize: '0.875rem',
                                    fontWeight: 600,
                                }}
                            />
                        </ListItemButton>
                    </ListItem>
                )}
            </Box>
    )

    return (
        <>
            {/* モバイル用: Temporary Drawer */}
            <Drawer
                variant="temporary"
                open={mobileOpen}
                onClose={onMobileClose}
                ModalProps={{ keepMounted: true }}
                className={styles.drawerMobile}
                sx={drawerSx}
            >
                {drawerContent}
            </Drawer>
            {/* デスクトップ用: Permanent Drawer */}
            <Drawer
                variant="permanent"
                className={styles.drawerDesktop}
                sx={drawerSx}
                open
            >
                {drawerContent}
            </Drawer>
        </>
    )
}
