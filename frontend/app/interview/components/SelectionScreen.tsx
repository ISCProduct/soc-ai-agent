'use client'

import { useEffect, type Dispatch, type KeyboardEvent, type SetStateAction } from 'react'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  LinearProgress,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import ApartmentIcon from '@mui/icons-material/Apartment'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import ArrowForwardIcon from '@mui/icons-material/ArrowForward'
import PsychologyIcon from '@mui/icons-material/Psychology'
import LightbulbIcon from '@mui/icons-material/Lightbulb'
import type { User } from '@/lib/auth'
import { interviewLimits } from '@/lib/interview'
import { PRIMARY, BG_LIGHT, POSITIONS } from '../constants'
import type { InterviewCompany, Position } from '../types'
import { resolveCompanyByName } from '../utils'

const COMPANY_RESOLVE_DEBOUNCE_MS = 400

function activateOnEnterOrSpace(event: KeyboardEvent, action: () => void) {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    action()
  }
}

export interface CompanyHints {
  style_tags: string[]
  top_questions: string[]
  company_brief?: string
}

export interface SelectionScreenProps {
  user: User
  onBack: () => void
  interviewCompany: InterviewCompany | null
  setInterviewCompany: Dispatch<SetStateAction<InterviewCompany | null>>
  companySourceTab: 'db' | 'web'
  setCompanySourceTab: Dispatch<SetStateAction<'db' | 'web'>>
  companySearch: string
  setCompanySearch: Dispatch<SetStateAction<string>>
  allCompanies: InterviewCompany[]
  setAllCompanies: Dispatch<SetStateAction<InterviewCompany[]>>
  companiesLoading: boolean
  webSearchResults: { name: string; description: string }[]
  setWebSearchResults: Dispatch<SetStateAction<{ name: string; description: string }[]>>
  webSearchLoading: boolean
  positionCategory: 'general' | 'sier'
  setPositionCategory: Dispatch<SetStateAction<'general' | 'sier'>>
  selectedPosition: Position
  setSelectedPosition: Dispatch<SetStateAction<Position>>
  companyHints: CompanyHints | null
  hintsLoading: boolean
  questionDurationSeconds: number
  onStartInterview: () => void
}

/**
 * SELECTION SCREEN (Step 1 of 3)
 * 志望企業・応募職種を選ぶ画面。データ取得（useEffect）は親コンポーネント側で行い、
 * このコンポーネントは受け取った props をそのまま描画する presentational component。
 */
export default function SelectionScreen({
  user,
  onBack,
  interviewCompany,
  setInterviewCompany,
  companySourceTab,
  setCompanySourceTab,
  companySearch,
  setCompanySearch,
  allCompanies,
  setAllCompanies,
  companiesLoading,
  webSearchResults,
  setWebSearchResults,
  webSearchLoading,
  positionCategory,
  setPositionCategory,
  selectedPosition,
  setSelectedPosition,
  companyHints,
  hintsLoading,
  questionDurationSeconds,
  onStartInterview,
}: SelectionScreenProps) {
  // 入力中の企業名解決は親の一覧取得と同様にデバウンスし、1文字ごとの API 連打を避ける。
  // allCompanies に名前一致がある場合は API を呼ばず、id:0 の仮選択を登録企業へ昇格させる。
  useEffect(() => {
    if (companySourceTab !== 'db') return
    const trimmed = companySearch.trim()
    if (!trimmed) return

    const local = allCompanies.find((c) => c.name === trimmed)
    if (local) {
      setInterviewCompany((prev) => {
        if (prev?.id === local.id && prev.name === local.name) return prev
        if (!prev || prev.name !== trimmed || prev.id === 0 || prev.id !== local.id) {
          return local
        }
        return prev
      })
      return
    }

    const timer = window.setTimeout(() => {
      void resolveCompanyByName(trimmed).then((resolved) => {
        setInterviewCompany((prev) => (prev?.name === trimmed ? resolved : prev))
      })
    }, COMPANY_RESOLVE_DEBOUNCE_MS)

    return () => window.clearTimeout(timer)
  }, [companySearch, companySourceTab, allCompanies, setInterviewCompany])

  /**
   * デバウンス待機中にロビーへ進むと resolve がキャンセルされ id:0 のまま残るため、
   * 開始直前に登録企業解決を確定する（Bugbot: Debounced resolve skips ID lookup）。
   */
  const handleStartInterview = async () => {
    const trimmed = (companySearch.trim() || interviewCompany?.name || '').trim()
    if (interviewCompany && interviewCompany.id === 0 && trimmed) {
      const local = allCompanies.find((c) => c.name === trimmed)
      const resolved = local ?? await resolveCompanyByName(trimmed, interviewCompany)
      setInterviewCompany(resolved)
    }
    onStartInterview()
  }

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: BG_LIGHT }}>
      {/* Header */}
      <Box component="header" sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: { xs: 2, sm: 3, lg: 10 }, py: 2, bgcolor: '#fff', borderBottom: '1px solid #e2e8f0' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ color: PRIMARY, display: 'flex', alignItems: 'center' }}>
            <PsychologyIcon sx={{ fontSize: 32 }} />
          </Box>
          <Typography sx={{ fontWeight: 700, fontSize: { xs: 16, sm: 20 }, color: '#0f172a' }}>InterviewAI</Typography>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <IconButton sx={{ bgcolor: '#f1f5f9', color: '#475569' }} size="small" onClick={onBack}>
            <ArrowBackIcon fontSize="small" />
          </IconButton>
          <Box sx={{ width: 40, height: 40, borderRadius: '50%', bgcolor: `${PRIMARY}30`, border: `1px solid ${PRIMARY}50`, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Typography sx={{ fontWeight: 700, color: PRIMARY, fontSize: 14 }}>
              {(user.name || 'U').charAt(0).toUpperCase()}
            </Typography>
          </Box>
        </Box>
      </Box>

      {/* Main */}
      <Box component="main" sx={{ display: 'flex', justifyContent: 'center', py: { xs: 3, sm: 5 }, px: { xs: 2, sm: 3, lg: 10 } }}>
        <Box sx={{ maxWidth: 896, width: '100%', display: 'flex', flexDirection: 'column', gap: 4 }}>

          {/* Step indicator + Title */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
              <Typography sx={{ color: PRIMARY, fontWeight: 600, fontSize: 13, textTransform: 'uppercase', letterSpacing: 1 }}>
                Step 1 / 3
              </Typography>
              <Box sx={{ height: 4, width: 96, bgcolor: '#e2e8f0', borderRadius: 9999, overflow: 'hidden' }}>
                <Box sx={{ height: '100%', width: '33%', bgcolor: PRIMARY }} />
              </Box>
            </Box>
            <Typography variant="h4" sx={{ fontWeight: 700, color: '#0f172a', fontSize: { xs: '1.4rem', sm: '2.125rem' } }}>練習する企業・職種を選ぶ</Typography>
            <Typography sx={{ color: '#64748b', fontSize: 15 }}>
              志望企業と職種を選択して、AIが面接内容をカスタマイズします。
            </Typography>
          </Box>

          {/* 3-col grid */}
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '2fr 1fr' }, gap: 4, alignItems: 'start' }}>

            {/* Left: Company + Position */}
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>

              {/* Company section */}
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0' }}>
                <Typography sx={{ fontWeight: 700, fontSize: 17, mb: 2 }}>志望企業</Typography>

                {/* Source tab toggle */}
                <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
                  <Button
                    size="small"
                    onClick={() => { setCompanySourceTab('db'); setWebSearchResults([]) }}
                    sx={{
                      px: 2, py: 0.6, borderRadius: 2, textTransform: 'none', fontWeight: 600, fontSize: 13,
                      bgcolor: companySourceTab === 'db' ? PRIMARY : '#f1f5f9',
                      color: companySourceTab === 'db' ? '#fff' : '#475569',
                      '&:hover': { bgcolor: companySourceTab === 'db' ? `${PRIMARY}e0` : '#e2e8f0' },
                    }}
                  >
                    🏢 企業管理から選択
                  </Button>
                  <Button
                    size="small"
                    onClick={() => { setCompanySourceTab('web'); setAllCompanies([]) }}
                    sx={{
                      px: 2, py: 0.6, borderRadius: 2, textTransform: 'none', fontWeight: 600, fontSize: 13,
                      bgcolor: companySourceTab === 'web' ? PRIMARY : '#f1f5f9',
                      color: companySourceTab === 'web' ? '#fff' : '#475569',
                      '&:hover': { bgcolor: companySourceTab === 'web' ? `${PRIMARY}e0` : '#e2e8f0' },
                    }}
                  >
                    🔍 WEB検索
                  </Button>
                </Box>

                {/* Search / direct input */}
                <Box sx={{ position: 'relative', mb: 2 }}>
                  <SearchIcon sx={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8', fontSize: 20 }} />
                  <Box
                    component="input"
                    value={companySearch}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                      const value = e.target.value
                      setCompanySearch(value)
                      if (companySourceTab === 'db' && value.trim()) {
                        const local = allCompanies.find((c) => c.name === value.trim())
                        if (local) {
                          setInterviewCompany(local)
                        } else {
                          // 即時は id=0 の仮選択。DB 解決は上記 useEffect でデバウンス
                          setInterviewCompany({ id: 0, name: value.trim() })
                        }
                      }
                    }}
                    placeholder={companySourceTab === 'web' ? '企業名を入力してWEB検索' : '企業名を入力、または下のリストから選択'}
                    sx={{
                      width: '100%', pl: '40px', pr: 2, py: 1.5,
                      bgcolor: '#f8fafc', border: '1px solid #e2e8f0',
                      borderRadius: 2, fontSize: 14, color: '#0f172a',
                      outline: 'none', boxSizing: 'border-box',
                      '&:focus': { borderColor: PRIMARY, boxShadow: `0 0 0 2px ${PRIMARY}20` },
                      fontFamily: 'inherit',
                    }}
                  />
                </Box>

                {/* Manual entry / unresolved company warning (#567) */}
                {interviewCompany && interviewCompany.id === 0 && interviewCompany.name.trim() && (
                  <Box sx={{ mb: 2, p: 1.5, borderRadius: 2, bgcolor: '#fff8e1', border: '1px solid #ffe082', display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                    <WarningAmberIcon sx={{ color: '#f9a825', fontSize: 18, mt: 0.2, flexShrink: 0 }} />
                    <Box>
                      <Typography sx={{ fontSize: 13, color: '#f57f17', fontWeight: 600 }}>
                        「{interviewCompany.name}」は企業管理に未登録です
                      </Typography>
                      <Typography sx={{ fontSize: 12, color: '#795548', mt: 0.5, lineHeight: 1.5 }}>
                        カスタム質問・深掘り追質問は、企業管理に登録された企業を選択した場合のみ利用できます。一般的な面接練習は可能です。
                      </Typography>
                    </Box>
                  </Box>
                )}
                {interviewCompany && interviewCompany.id > 0 && companySourceTab !== 'db' && (
                  <Box sx={{ mb: 2, p: 1.5, borderRadius: 2, bgcolor: `${PRIMARY}08`, border: `1px solid ${PRIMARY}30`, display: 'flex', alignItems: 'center', gap: 1 }}>
                    <CheckCircleIcon sx={{ color: PRIMARY, fontSize: 18 }} />
                    <Typography sx={{ fontSize: 13, color: PRIMARY, fontWeight: 600 }}>
                      企業管理の「{interviewCompany.name}」と一致しました（カスタム質問・深掘りが有効）
                    </Typography>
                  </Box>
                )}

                {/* DB mode: Company chips from DB */}
                {companySourceTab === 'db' && (
                  <>
                    <Typography sx={{ fontSize: 12, color: '#94a3b8', mb: 1 }}>登録企業から選択</Typography>
                    {companiesLoading ? (
                      <LinearProgress sx={{ borderRadius: 1 }} />
                    ) : (
                      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5 }}>
                        {/* allCompanies はサーバ側検索済みのため、追加のクライアントフィルタは不要 */}
                        {allCompanies.map(c => {
                          const isSelected = interviewCompany?.id === c.id
                          return (
                            <Button
                              key={c.id}
                              size="small"
                              onClick={() => { setInterviewCompany(c); setCompanySearch(c.name) }}
                              startIcon={isSelected ? <CheckCircleIcon sx={{ fontSize: '16px !important' }} /> : undefined}
                              sx={{
                                px: 2, py: 0.8, borderRadius: 2, fontWeight: 500, fontSize: 13,
                                textTransform: 'none',
                                bgcolor: isSelected ? PRIMARY : '#f1f5f9',
                                color: isSelected ? '#fff' : '#475569',
                                '&:hover': { bgcolor: isSelected ? `${PRIMARY}e0` : '#e2e8f0' },
                              }}
                            >
                              {c.name}
                            </Button>
                          )
                        })}
                        {allCompanies.length === 0 && !companiesLoading && !companySearch.trim() && (
                          <Typography sx={{ color: '#94a3b8', fontSize: 13 }}>登録企業が見つかりません</Typography>
                        )}
                      </Box>
                    )}
                  </>
                )}

                {/* WEB mode: web search results */}
                {companySourceTab === 'web' && (
                  <>
                    <Typography sx={{ fontSize: 12, color: '#94a3b8', mb: 1 }}>
                      {companySearch.trim() ? 'WEB検索結果' : 'キーワードを入力してください'}
                    </Typography>
                    {webSearchLoading ? (
                      <LinearProgress sx={{ borderRadius: 1 }} />
                    ) : (
                      <Stack spacing={1} role="listbox" aria-label="WEB検索結果">
                        {webSearchResults.map((result, i) => {
                          const isSelected = interviewCompany?.name === result.name
                          const selectResult = () => {
                            setCompanySearch(result.name)
                            const local = allCompanies.find((c) => c.name === result.name)
                            if (local) {
                              setInterviewCompany({ ...local, description: result.description })
                              return
                            }
                            setInterviewCompany({ id: 0, name: result.name, description: result.description })
                            void resolveCompanyByName(result.name, { description: result.description }).then((resolved) => {
                              setInterviewCompany((prev) =>
                                prev?.name === result.name ? resolved : prev,
                              )
                            })
                          }
                          return (
                            <Box
                              key={i}
                              role="option"
                              aria-selected={isSelected}
                              tabIndex={0}
                              onClick={selectResult}
                              onKeyDown={(e) => activateOnEnterOrSpace(e, selectResult)}
                              sx={{
                                p: 1.5, borderRadius: 2, cursor: 'pointer',
                                border: `2px solid ${isSelected ? PRIMARY : '#e2e8f0'}`,
                                bgcolor: isSelected ? `${PRIMARY}08` : '#f8fafc',
                                transition: 'all 0.15s',
                                '&:hover': { borderColor: PRIMARY, bgcolor: `${PRIMARY}05` },
                                '&:focus-visible': { outline: `2px solid ${PRIMARY}`, outlineOffset: 2 },
                              }}
                            >
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                {isSelected && <CheckCircleIcon sx={{ color: PRIMARY, fontSize: 16, flexShrink: 0 }} />}
                                <Typography sx={{ fontWeight: 600, fontSize: 14, color: '#0f172a' }}>{result.name}</Typography>
                              </Box>
                              {result.description && (
                                <Typography sx={{ fontSize: 12, color: '#64748b', mt: 0.5, lineHeight: 1.5 }}>
                                  {result.description.slice(0, 100)}{result.description.length > 100 ? '...' : ''}
                                </Typography>
                              )}
                            </Box>
                          )
                        })}
                        {webSearchResults.length === 0 && companySearch.trim() && !webSearchLoading && (
                          <Typography sx={{ color: '#94a3b8', fontSize: 13 }}>検索結果が見つかりません</Typography>
                        )}
                      </Stack>
                    )}
                  </>
                )}
              </Paper>

              {/* Position section */}
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0' }}>
                <Typography sx={{ fontWeight: 700, fontSize: 17, mb: 2 }}>応募職種</Typography>

                {/* Category tab */}
                <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
                  <Button
                    size="small"
                    onClick={() => {
                      setPositionCategory('general')
                      if (selectedPosition.category === 'sier') setSelectedPosition(POSITIONS[0])
                    }}
                    sx={{
                      px: 2, py: 0.6, borderRadius: 2, textTransform: 'none', fontWeight: 600, fontSize: 13,
                      bgcolor: positionCategory === 'general' ? PRIMARY : '#f1f5f9',
                      color: positionCategory === 'general' ? '#fff' : '#475569',
                      '&:hover': { bgcolor: positionCategory === 'general' ? `${PRIMARY}e0` : '#e2e8f0' },
                    }}
                  >
                    💼 一般・IT職種
                  </Button>
                  <Button
                    size="small"
                    onClick={() => {
                      setPositionCategory('sier')
                      if (selectedPosition.category === 'general') {
                        const firstSier = POSITIONS.find((p) => p.category === 'sier')
                        if (firstSier) setSelectedPosition(firstSier)
                      }
                    }}
                    sx={{
                      px: 2, py: 0.6, borderRadius: 2, textTransform: 'none', fontWeight: 600, fontSize: 13,
                      bgcolor: positionCategory === 'sier' ? PRIMARY : '#f1f5f9',
                      color: positionCategory === 'sier' ? '#fff' : '#475569',
                      '&:hover': { bgcolor: positionCategory === 'sier' ? `${PRIMARY}e0` : '#e2e8f0' },
                    }}
                  >
                    🏗️ SIer職種
                  </Button>
                </Box>

                <Box
                  role="radiogroup"
                  aria-label="応募職種"
                  sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1.5 }}
                >
                  {POSITIONS.filter(p => p.category === positionCategory).map(pos => {
                    const isSelected = selectedPosition.id === pos.id
                    return (
                      <Box
                        key={pos.id}
                        role="radio"
                        aria-checked={isSelected}
                        tabIndex={0}
                        onClick={() => setSelectedPosition(pos)}
                        onKeyDown={(e) => activateOnEnterOrSpace(e, () => setSelectedPosition(pos))}
                        sx={{
                          position: 'relative', display: 'flex', alignItems: 'center', gap: 1.5,
                          p: 2, borderRadius: 2, cursor: 'pointer',
                          border: `2px solid ${isSelected ? PRIMARY : 'transparent'}`,
                          bgcolor: isSelected ? `${PRIMARY}08` : '#f8fafc',
                          transition: 'all 0.15s',
                          '&:hover': { borderColor: isSelected ? PRIMARY : '#cbd5e1' },
                          '&:focus-visible': { outline: `2px solid ${PRIMARY}`, outlineOffset: 2 },
                        }}
                      >
                        <Typography sx={{ fontSize: 22 }}>{pos.icon}</Typography>
                        <Box sx={{ flex: 1 }}>
                          <Typography sx={{ fontWeight: 700, fontSize: 14, color: '#0f172a' }}>{pos.title}</Typography>
                          <Typography sx={{ fontSize: 12, color: '#94a3b8' }}>{pos.department}</Typography>
                        </Box>
                        <Box sx={{ position: 'absolute', right: 12, top: '50%', transform: 'translateY(-50%)' }}>
                          {isSelected
                            ? <CheckCircleIcon sx={{ color: PRIMARY, fontSize: 20 }} />
                            : <Box sx={{ width: 18, height: 18, borderRadius: '50%', border: '2px solid #cbd5e1' }} />
                          }
                        </Box>
                      </Box>
                    )
                  })}
                </Box>
              </Paper>
            </Box>

            {/* Right: Summary + CTA */}
            <Box sx={{ position: { md: 'sticky' }, top: 32, display: 'flex', flexDirection: 'column', gap: 3 }}>
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `1px solid ${PRIMARY}30`, bgcolor: `${PRIMARY}05` }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 3 }}>
                  <Box sx={{ width: 48, height: 48, borderRadius: 2, bgcolor: '#fff', border: '1px solid #e2e8f0', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <ApartmentIcon sx={{ color: PRIMARY }} />
                  </Box>
                  <Box>
                    <Typography sx={{ fontWeight: 700, fontSize: 15 }}>{interviewCompany?.name || '企業未選択'}</Typography>
                    <Typography sx={{ fontSize: 13, color: '#64748b' }}>{interviewCompany?.industry || '業種未設定'}</Typography>
                  </Box>
                </Box>

                <Stack spacing={2.5}>
                  <Box>
                    <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: 1, mb: 0.5 }}>応募ポジション</Typography>
                    <Typography sx={{ fontWeight: 600, fontSize: 15 }}>{selectedPosition.title}</Typography>
                  </Box>
                  <Box>
                    <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: 1, mb: 0.5 }}>企業概要</Typography>
                    <Typography sx={{ fontSize: 13, color: '#64748b', lineHeight: 1.7 }}>
                      {interviewCompany?.description
                        ? interviewCompany.description.slice(0, 120) + (interviewCompany.description.length > 120 ? '...' : '')
                        : '企業を選択すると詳細が表示されます。'}
                    </Typography>
                  </Box>
                  <Box sx={{ pt: 2, borderTop: `1px solid ${PRIMARY}15` }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                      <Typography sx={{ fontSize: 14 }}>⏱</Typography>
                      <Typography sx={{ fontSize: 13, color: '#475569' }}>所要時間: {interviewLimits.maxMinutes}分</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Typography sx={{ fontSize: 14 }}>❓</Typography>
                      <Typography sx={{ fontSize: 13, color: '#475569' }}>
                        {selectedPosition.questions}問（1問あたり約{Math.round(questionDurationSeconds / 60)}分）
                      </Typography>
                    </Box>
                  </Box>
                </Stack>
              </Paper>

              {/* 企業別面接傾向ヒント */}
              {interviewCompany && (
                <Paper elevation={0} sx={{ p: 2.5, borderRadius: 2, border: '1px solid #fcd34d', bgcolor: '#fffbeb' }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5 }}>
                    <LightbulbIcon sx={{ fontSize: 18, color: '#d97706' }} />
                    <Typography sx={{ fontWeight: 700, fontSize: 14, color: '#92400e' }}>
                      {interviewCompany.name} の面接傾向
                    </Typography>
                  </Box>
                  {hintsLoading ? (
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <CircularProgress size={14} sx={{ color: '#d97706' }} />
                      <Typography sx={{ fontSize: 12, color: '#92400e' }}>共有キャッシュから読み込み中...</Typography>
                    </Box>
                  ) : companyHints && (companyHints.style_tags.length > 0 || companyHints.top_questions.length > 0 || companyHints.company_brief) ? (
                    <Stack spacing={1.5}>
                      {companyHints.company_brief ? (
                        <Box>
                          <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#b45309', mb: 0.5 }}>企業スナップショット（DB）</Typography>
                          <Typography sx={{ fontSize: 12, color: '#78350f', whiteSpace: 'pre-wrap', lineHeight: 1.5 }}>
                            {companyHints.company_brief}
                          </Typography>
                        </Box>
                      ) : null}
                      {companyHints.style_tags.length > 0 && (
                        <Box>
                          <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#b45309', mb: 0.5 }}>面接スタイル</Typography>
                          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                            {companyHints.style_tags.map((tag, i) => (
                              <Chip key={i} label={tag} size="small" sx={{ bgcolor: '#fef3c7', color: '#92400e', fontWeight: 600, fontSize: 11, border: '1px solid #fcd34d' }} />
                            ))}
                          </Box>
                        </Box>
                      )}
                      {companyHints.top_questions.length > 0 && (
                        <Box>
                          <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#b45309', mb: 0.5 }}>よく聞かれる質問</Typography>
                          <Stack spacing={0.5}>
                            {companyHints.top_questions.map((q, i) => (
                              <Box key={i} sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
                                <Typography sx={{ fontSize: 11, fontWeight: 700, color: PRIMARY, minWidth: 16 }}>{i + 1}.</Typography>
                                <Typography sx={{ fontSize: 12, color: '#78350f', lineHeight: 1.5 }}>{q}</Typography>
                              </Box>
                            ))}
                          </Stack>
                        </Box>
                      )}
                    </Stack>
                  ) : (
                    <Typography sx={{ fontSize: 12, color: '#92400e' }}>
                      共有キャッシュに企業情報があるとスナップショットを表示します（都度Web検索はしません）。
                    </Typography>
                  )}
                </Paper>
              )}

              <Button
                variant="contained"
                fullWidth
                endIcon={<ArrowForwardIcon />}
                disabled={!interviewCompany}
                onClick={() => { void handleStartInterview() }}
                sx={{
                  bgcolor: PRIMARY, '&:hover': { bgcolor: `${PRIMARY}e0` },
                  borderRadius: 2, py: 1.8, fontWeight: 700, fontSize: 16,
                  textTransform: 'none',
                  boxShadow: `0 8px 24px ${PRIMARY}30`,
                  '&:disabled': { bgcolor: '#e2e8f0', color: '#94a3b8', boxShadow: 'none' },
                }}
              >
                面接を開始する
              </Button>
              <Typography sx={{ fontSize: 12, textAlign: 'center', color: '#94a3b8' }}>
                開始すると<Box component="a" href="#" sx={{ textDecoration: 'underline', color: 'inherit' }}>利用規約</Box>に同意したことになります
              </Typography>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  )
}
