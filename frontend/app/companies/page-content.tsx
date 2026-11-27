'use client'

import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import NextLink from 'next/link'
import {
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Container,
  IconButton,
  MenuItem,
  Pagination,
  Skeleton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import { Alert } from '@/components/common/Alert'
import Favorite from '@mui/icons-material/Favorite'
import FavoriteBorder from '@mui/icons-material/FavoriteBorder'
import {
  COMPANY_SEARCH_PAGE_SIZE,
  fetchFavoriteStates,
  fetchSearchOptions,
  searchCompanies,
  toggleFavorite,
  type CompanyFavoriteState,
  type CompanySearchFilter,
  type CompanySearchOptions,
  type CompanySearchResult,
} from '@/lib/company-search'
import { ProvenanceBadge } from '@/components/company/ProvenanceBadge'
import { classifyProvenance } from '@/lib/company/provenance'

/**
 * 企業を探す（SOCAIAGENT-612）。業界・地域（都道府県）・職種・名前で、自分の条件で企業を探す。
 * AI の推薦とは別の、中立な一覧。並びは企業名順で、おすすめ順や課金順ではない。
 * お気に入りは推薦（マッチング結果）に付くので、推薦に出た企業だけハートを出す。
 */
export default function CompanySearchPage() {
  const [options, setOptions] = useState<CompanySearchOptions | null>(null)
  const [optionsError, setOptionsError] = useState(false)
  // 入力中の値と、検索に使う値を分ける（名前は「検索」を押したときだけ反映する）
  const [nameInput, setNameInput] = useState('')
  const [filter, setFilter] = useState<CompanySearchFilter>({ page: 1 })
  const [result, setResult] = useState<CompanySearchResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [favorites, setFavorites] = useState<Record<number, CompanyFavoriteState>>({})
  // 送信中の企業。企業ごとに持つ（別の企業を押しても、送信中のボタンを押せるようにしない）
  const [toggling, setToggling] = useState<ReadonlySet<number>>(() => new Set())
  // 同じ描画のうちに続けて押されても弾けるよう、判定は ref で行う（state は表示用）
  const togglingRef = useRef<Set<number>>(new Set())
  const [favoriteError, setFavoriteError] = useState('')
  // お気に入りの状態を取れなかった（推薦が無いのとは区別して、再試行を出す）
  const [favoritesLoadFailed, setFavoritesLoadFailed] = useState(false)

  const loadOptions = useCallback(async () => {
    setOptionsError(false)
    try {
      setOptions(await fetchSearchOptions())
    } catch {
      setOptionsError(true)
    }
  }, [])

  useEffect(() => {
    void loadOptions()
  }, [loadOptions])

  // 条件を続けて変えたとき、前の条件の遅い応答で一覧を上書きしない
  const seq = useRef(0)
  const controller = useRef<AbortController | null>(null)

  // お気に入りの状態は補助。取れなくても一覧は出し、失敗したことと再試行を出す
  const loadFavorites = useCallback(async (companyIds: number[], mine: number) => {
    setFavoritesLoadFailed(false)
    try {
      const states = await fetchFavoriteStates(companyIds)
      if (mine === seq.current) setFavorites(Object.fromEntries(states.map((s) => [s.company_id, s])))
    } catch {
      if (mine !== seq.current) return
      setFavorites({})
      setFavoritesLoadFailed(true)
    }
  }, [])

  const load = useCallback(async (f: CompanySearchFilter) => {
    const mine = ++seq.current
    controller.current?.abort()
    const ac = new AbortController()
    controller.current = ac
    setLoading(true)
    setError('')
    try {
      const r = await searchCompanies(f, ac.signal)
      if (mine !== seq.current) return
      setResult(r)
      void loadFavorites(r.companies.map((c) => c.id), mine)
    } catch {
      if (mine !== seq.current) return
      setError('企業を読み込めませんでした。通信状況を確認して、もう一度お試しください')
    } finally {
      if (mine === seq.current) setLoading(false)
    }
  }, [loadFavorites])

  useEffect(() => {
    void load(filter)
    return () => controller.current?.abort()
  }, [filter, load])

  // 条件を変えたら1ページ目に戻す
  const update = (patch: Partial<CompanySearchFilter>) => setFilter((prev) => ({ ...prev, ...patch, page: 1 }))

  const submitName = (e: FormEvent) => {
    e.preventDefault()
    update({ name: nameInput.trim() || undefined })
  }

  const clearAll = () => {
    setNameInput('')
    setFilter({ page: 1 })
  }

  const onToggleFavorite = async (state: CompanyFavoriteState) => {
    if (togglingRef.current.has(state.company_id)) return
    togglingRef.current.add(state.company_id)
    setToggling(new Set(togglingRef.current))
    setFavoriteError('')
    try {
      await toggleFavorite(state.match_id)
      setFavorites((prev) => ({ ...prev, [state.company_id]: { ...state, is_favorited: !state.is_favorited } }))
    } catch {
      setFavoriteError('お気に入りを切り替えられませんでした。もう一度お試しください')
    } finally {
      togglingRef.current.delete(state.company_id)
      setToggling(new Set(togglingRef.current))
    }
  }

  const filtered = Boolean(filter.name || filter.industry || filter.prefecture || filter.jobCategoryId)
  const pageCount = result ? Math.max(1, Math.ceil(result.total / COMPANY_SEARCH_PAGE_SIZE)) : 1

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
      <Container maxWidth="md" sx={{ py: 3, pb: { xs: 12, md: 4 } }}>
        <Typography variant="h5" component="h1" fontWeight="bold" gutterBottom>企業を探す</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          業界・地域・職種を選んで、自分の条件で企業を探せます。並びは企業名順で、おすすめ順ではありません。
        </Typography>

        <Box component="form" onSubmit={submitName} role="search" aria-label="企業の検索条件" sx={{ mb: 2 }}>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} sx={{ mb: 1.5 }}>
            <TextField
              size="small"
              label="企業名"
              value={nameInput}
              onChange={(e) => setNameInput(e.target.value)}
              inputProps={{ maxLength: 100 }}
              sx={{ flex: 1 }}
            />
            <Button type="submit" variant="contained">検索</Button>
          </Stack>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
            <TextField select size="small" label="業界" value={filter.industry ?? ''} onChange={(e) => update({ industry: e.target.value || undefined })} sx={{ minWidth: 180 }} disabled={!options}>
              <MenuItem value="">すべての業界</MenuItem>
              {options?.industries.map((i) => <MenuItem key={i} value={i}>{i}</MenuItem>)}
            </TextField>
            <TextField select size="small" label="地域" value={filter.prefecture ?? ''} onChange={(e) => update({ prefecture: e.target.value || undefined })} sx={{ minWidth: 160 }} disabled={!options}>
              <MenuItem value="">すべての地域</MenuItem>
              {options?.prefectures.map((p) => <MenuItem key={p} value={p}>{p}</MenuItem>)}
            </TextField>
            <TextField select size="small" label="職種" value={filter.jobCategoryId ?? ''} onChange={(e) => update({ jobCategoryId: e.target.value === '' ? undefined : Number(e.target.value) })} sx={{ minWidth: 180 }} disabled={!options}>
              <MenuItem value="">すべての職種</MenuItem>
              {options?.job_categories.map((c) => <MenuItem key={c.id} value={c.id}>{c.name}</MenuItem>)}
            </TextField>
          </Stack>
        </Box>

        {optionsError && (
          <Alert role="alert" severity="warning" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={() => void loadOptions()}>再読み込み</Button>}>
            絞り込みの選択肢を読み込めませんでした。企業名での検索はできます。
          </Alert>
        )}
        {error && (
          <Alert severity="error" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={() => void load(filter)}>再読み込み</Button>}>
            {error}
          </Alert>
        )}
        {favoriteError && <Alert severity="error" sx={{ mb: 2 }}>{favoriteError}</Alert>}
        {favoritesLoadFailed && result && !error && (
          <Alert role="alert"
            severity="warning"
            sx={{ mb: 2 }}
            action={<Button color="inherit" size="small" onClick={() => void loadFavorites(result.companies.map((c) => c.id), seq.current)}>再読み込み</Button>}
          >
            お気に入りの状態を読み込めませんでした。企業の一覧は見られます。
          </Alert>
        )}

        {loading && !result && (
          <Stack spacing={1.5} aria-label="企業を読み込み中">
            {[0, 1, 2].map((i) => <Skeleton key={i} variant="rounded" height={88} />)}
          </Stack>
        )}

        {result && !error && (
          <>
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1, gap: 1 }}>
              <Typography variant="body2" role="status">
                {loading ? '読み込み中…' : `${result.total}社`}
              </Typography>
              {filtered && <Button size="small" onClick={clearAll}>条件をクリア</Button>}
            </Box>
            {result.companies.length === 0 ? (
              <Typography color="text.secondary">
                {filtered ? '条件に合う企業がありません。条件を変えてみてください。' : 'まだ企業が登録されていません。'}
              </Typography>
            ) : (
              <Stack component="ul" spacing={1.5} sx={{ listStyle: 'none', p: 0, m: 0 }}>
                {result.companies.map((c) => {
                  const fav = favorites[c.id]
                  return (
                    <Box component="li" key={c.id}>
                      <Card variant="outlined" sx={{ display: 'flex', alignItems: 'stretch' }}>
                        <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                        <CardActionArea component={NextLink} href={`/company/${c.id}`} sx={{ flex: 1 }}>
                          <CardContent>
                            <Typography fontWeight={700}>{c.name}</Typography>
                            <Typography variant="caption" color="text.secondary">
                              {[c.industry, c.location].filter(Boolean).join('　')}
                            </Typography>
                            {c.description && (
                              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                                {c.description}
                              </Typography>
                            )}
                          </CardContent>
                        </CardActionArea>
                        {/* AI 補完の紹介文を確認済みの事実に見せない（詳細画面と同じ判定）。ツールチップを開けるようリンクの外に置く */}
                        {classifyProvenance(c)?.kind === 'ai' && (
                          <Box sx={{ px: 2, pb: 1.5 }}>
                            <ProvenanceBadge provenance={c} target="企業の紹介・業界・所在地" />
                          </Box>
                        )}
                        </Box>
                        {fav && (
                          <Box sx={{ display: 'flex', alignItems: 'center', pr: 1 }}>
                            <Tooltip title={fav.is_favorited ? 'お気に入りから外す' : 'お気に入りに入れる'}>
                              <IconButton
                                aria-label={`${c.name}を${fav.is_favorited ? 'お気に入りから外す' : 'お気に入りに入れる'}`}
                                aria-pressed={fav.is_favorited}
                                disabled={toggling.has(c.id)}
                                onClick={() => void onToggleFavorite(fav)}
                              >
                                {fav.is_favorited ? <Favorite color="error" /> : <FavoriteBorder />}
                              </IconButton>
                            </Tooltip>
                          </Box>
                        )}
                      </Card>
                    </Box>
                  )
                })}
              </Stack>
            )}
            {pageCount > 1 && (
              <Box sx={{ display: 'flex', justifyContent: 'center', mt: 3 }}>
                <Pagination
                  count={pageCount}
                  page={filter.page}
                  onChange={(_, page) => setFilter((prev) => ({ ...prev, page }))}
                  // 新しい条件の読み込み中は、古い結果のページ数で移動させない
                  disabled={loading}
                  aria-label="ページ"
                />
              </Box>
            )}
            <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 2 }}>
              お気に入りは、AI 診断のマッチング結果に出た企業で使えます（ハートが出ている企業）。
            </Typography>
          </>
        )}
      </Container>
    </Box>
  )
}
