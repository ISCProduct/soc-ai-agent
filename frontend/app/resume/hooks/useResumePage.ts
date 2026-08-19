'use client'

import { useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { authService } from '@/lib/auth'
import type { WeightScore } from '@/components/ScoreUpdateBanner'
import type { CompanyCandidate, ReviewResult } from '../types'
import {
  isAnnotatedPdfResponse,
  mapDbCompanyResults,
  mapWebSearchResults,
  parseApiErrorMessage,
} from '../utils'

/**
 * 履歴書レビューページの state・副作用・イベントハンドラ。
 */
export function useResumePage() {
  const searchParams = useSearchParams()
  const [userId, setUserId] = useState('')
  const [sessionId, setSessionId] = useState('')
  const [sourceType, setSourceType] = useState('pdf')
  const [sourceUrl, setSourceUrl] = useState('')
  const [companyName, setCompanyName] = useState('')
  const [companyQuery, setCompanyQuery] = useState('')
  const [companyCandidates, setCompanyCandidates] = useState<CompanyCandidate[]>([])
  const [companyValidated, setCompanyValidated] = useState(false)
  const [companySearchLoading, setCompanySearchLoading] = useState(false)
  const [companySearchError, setCompanySearchError] = useState('')
  const [selectedCompanyMeta, setSelectedCompanyMeta] = useState<CompanyCandidate | null>(null)
  const [jobTitle, setJobTitle] = useState('')
  const [candidateType, setCandidateType] = useState('new_grad')
  const [file, setFile] = useState<File | null>(null)
  const [documentId, setDocumentId] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)
  const [reviewLoading, setReviewLoading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const [reviewError, setReviewError] = useState('')
  const [annotateError, setAnnotateError] = useState('')
  const [review, setReview] = useState<ReviewResult | null>(null)
  const [ragReport, setRagReport] = useState('')
  const [scoresBefore, setScoresBefore] = useState<WeightScore[] | null>(null)
  const [scoresAfter, setScoresAfter] = useState<WeightScore[] | null>(null)
  const reviewAbortRef = useRef<AbortController | null>(null)

  const prefilledCompany = searchParams.get('company_name') || ''
  const prefilledIndustry = searchParams.get('industry') || ''

  const checkAnnotatedPdfAvailable = async (id: number) => {
    try {
      const response = await fetch(`/api/resume/annotated?document_id=${id}`, {
        headers: { Range: 'bytes=0-0', ...authService.getUserFetchHeaders() },
      })
      if (!response.ok) return false
      return isAnnotatedPdfResponse(
        response.headers.get('content-type') || '',
        response.headers.get('content-disposition') || '',
        response.status,
        response.headers.get('content-length'),
        response.headers.get('content-range'),
      )
    } catch {
      return false
    }
  }

  useEffect(() => {
    const user = authService.getStoredUser()
    if (user?.user_id) {
      setUserId(String(user.user_id))
    }
    if (typeof window !== 'undefined') {
      const storedSession =
        sessionStorage.getItem('chatSessionId') ||
        localStorage.getItem('currentSessionId') ||
        ''
      setSessionId(storedSession)
    }
    if (prefilledCompany) {
      setCompanyName(prefilledCompany)
      setCompanyQuery(prefilledCompany)
      setCompanyValidated(false)
      setSelectedCompanyMeta(null)
    }
  }, [prefilledCompany])

  useEffect(() => {
    return () => {
      reviewAbortRef.current?.abort()
    }
  }, [])

  const selectCompany = (candidate: CompanyCandidate) => {
    setCompanyName(candidate.name)
    setCompanyQuery(candidate.name)
    setCompanyValidated(true)
    setSelectedCompanyMeta(candidate)
    setCompanyCandidates([])
    setCompanySearchError('')
  }

  const clearSelectedCompany = () => {
    setCompanyName('')
    setCompanyQuery('')
    setCompanyValidated(false)
    setSelectedCompanyMeta(null)
    setCompanyCandidates([])
    setCompanySearchError('')
  }

  const handleCompanyQueryChange = (value: string) => {
    setCompanyQuery(value)
    setCompanyValidated(false)
    setSelectedCompanyMeta(null)
    setCompanyName('')
  }

  const handleSearchCompanies = async (includeWebSearch: boolean) => {
    const q = companyQuery.trim()
    if (!q) {
      setCompanySearchError('企業名を入力してください')
      return
    }
    setCompanySearchLoading(true)
    setCompanySearchError('')
    setCompanyCandidates([])
    try {
      if (!includeWebSearch) {
        const res = await fetch(`/api/companies?name=${encodeURIComponent(q)}&limit=5`, { cache: 'no-store' })
        if (!res.ok) {
          const errText = await res.text()
          throw new Error(parseApiErrorMessage(errText, 'DB検索に失敗しました'))
        }
        const data = await res.json()
        const list = mapDbCompanyResults(data)
        setCompanyCandidates(list)
        if (list.length === 0) {
          setCompanySearchError('DBに該当企業がありません。「WEBで実在確認」を試してください')
        }
        return
      }

      // WEB: 候補一覧 API（DB優先 + WEB補完）。単体 validate より候補選択しやすい
      const res = await fetch(`/api/companies/web-search?q=${encodeURIComponent(q)}`, { cache: 'no-store' })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error(parseApiErrorMessage(JSON.stringify(data), 'WEB検索に失敗しました'))
      }
      const list = mapWebSearchResults(data)
      setCompanyCandidates(list)
      if (list.length === 0) {
        setCompanyValidated(false)
        setSelectedCompanyMeta(null)
        setCompanyName('')
        setCompanySearchError('候補が見つかりませんでした。企業名を変えて再検索するか、職種のみでレビューしてください')
      }
    } catch (err) {
      setCompanySearchError(err instanceof Error ? err.message : '企業検索に失敗しました')
    } finally {
      setCompanySearchLoading(false)
    }
  }

  const handleUpload = async () => {
    // 別文書の再アップロード時、直前のレビュー結果の残存や
    // 進行中ストリームによる上書きを防ぐため、レビュー関連stateを初期化する
    reviewAbortRef.current?.abort()
    setUploadError('')
    setReview(null)
    setRagReport('')
    setScoresBefore(null)
    setScoresAfter(null)
    setAnnotateError('')
    setLoading(true)
    try {
      if (!userId) {
        throw new Error('user_id が取得できません。ログインしてください。')
      }
      const formData = new FormData()
      formData.append('user_id', userId)
      if (sessionId) {
        formData.append('session_id', sessionId)
      }
      formData.append('source_type', sourceType)
      if (sourceUrl) {
        formData.append('source_url', sourceUrl)
      }
      if (file) {
        formData.append('file', file)
      }

      const response = await fetch('/api/resume/upload', {
        method: 'POST',
        headers: authService.getUserFetchHeaders(),
        body: formData,
      })
      if (!response.ok) {
        const errText = await response.text()
        throw new Error(parseApiErrorMessage(errText, 'Upload failed'))
      }
      const data = await response.json()
      setDocumentId(data?.document?.id ?? null)
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setLoading(false)
    }
  }

  const handleReview = async () => {
    if (!documentId) {
      setReviewError('document_id が未設定です')
      return
    }
    if (!companyName.trim() && !jobTitle.trim()) {
      setReviewError('企業名が未入力の場合は応募職種を入力してください')
      return
    }
    if (companyName.trim() && !companyValidated) {
      setReviewError('企業を検索して候補から選択するか、「WEBで実在確認」を実行してください')
      return
    }

    reviewAbortRef.current?.abort()
    const abortController = new AbortController()
    reviewAbortRef.current = abortController

    setReviewError('')
    setAnnotateError('')
    setReview(null)
    setRagReport('')
    setScoresAfter(null)
    setReviewLoading(true)

    if (userId && sessionId) {
      try {
        const res = await fetch(`/api/user/weight-scores?user_id=${userId}&session_id=${encodeURIComponent(sessionId)}`, {
          signal: abortController.signal,
        })
        const data = await res.json()
        setScoresBefore(data.weight_scores ?? null)
      } catch (err) {
        if (err instanceof DOMException && err.name === 'AbortError') {
          setReviewLoading(false)
          return
        }
        /* ignore */
      }
    }

    try {
      const response = await fetch(`/api/resume/review/stream?document_id=${documentId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({
          company_name: companyName,
          job_title: jobTitle,
          candidate_type: candidateType,
        }),
        signal: abortController.signal,
      })

      if (!response.ok || !response.body) {
        const errText = await response.text()
        throw new Error(errText || 'Review failed')
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        if (abortController.signal.aborted) {
          await reader.cancel().catch(() => undefined)
          break
        }

        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue

          let data: {
            type?: string
            text?: string
            review?: ReviewResult['review']
            items?: ReviewResult['items']
            annotated_available?: boolean
            message?: string
          }
          try {
            data = JSON.parse(line.slice(6))
          } catch {
            // 不完全・不正な行はスキップ
            continue
          }

          if (data.type === 'chunk') {
            setRagReport((prev) => prev + (data.text ?? ''))
          } else if (data.type === 'complete') {
            const backendAnnotated = data.annotated_available === true
            const annotatedAvailable = backendAnnotated || (documentId ? await checkAnnotatedPdfAvailable(documentId) : false)
            if (abortController.signal.aborted) return
            setReview({
              review: data.review as ReviewResult['review'],
              items: data.items ?? [],
              annotated_available: annotatedAvailable,
            })
            if (userId && sessionId) {
              try {
                const res = await fetch(`/api/user/weight-scores?user_id=${userId}&session_id=${encodeURIComponent(sessionId)}`, {
                  signal: abortController.signal,
                })
                const scoreData = await res.json()
                if (!abortController.signal.aborted) {
                  setScoresAfter(scoreData.weight_scores ?? null)
                }
              } catch (err) {
                if (err instanceof DOMException && err.name === 'AbortError') return
                /* ignore */
              }
            }
          } else if (data.type === 'annotate_error') {
            setAnnotateError(String(data.message ?? ''))
          } else if (data.type === 'error') {
            throw new Error(String(data.message ?? 'Review failed'))
          }
        }
      }
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        return
      }
      setReviewError(err instanceof Error ? err.message : 'Review failed')
    } finally {
      if (!abortController.signal.aborted) {
        setReviewLoading(false)
      }
    }
  }

  const handleDownload = () => {
    if (!documentId) {
      setReviewError('document_id が未設定です')
      return
    }
    void (async () => {
      try {
        setReviewError('')
        const response = await fetch(`/api/resume/annotated?document_id=${documentId}`, {
          headers: authService.getUserFetchHeaders(),
        })
        if (!response.ok) {
          const errText = await response.text()
          throw new Error(errText || 'Download failed')
        }
        const blob = await response.blob()
        const url = URL.createObjectURL(blob)
        window.open(url, '_blank')
        setTimeout(() => URL.revokeObjectURL(url), 60_000)
      } catch (err) {
        setReviewError(err instanceof Error ? err.message : 'Download failed')
      }
    })()
  }

  return {
    prefilledCompany,
    prefilledIndustry,
    sourceType,
    setSourceType,
    sourceUrl,
    setSourceUrl,
    file,
    setFile,
    loading,
    uploadError,
    documentId,
    handleUpload,
    companyQuery,
    handleCompanyQueryChange,
    companyName,
    companyValidated,
    companySearchLoading,
    companySearchError,
    selectedCompanyMeta,
    companyCandidates,
    selectCompany,
    clearSelectedCompany,
    handleSearchCompanies,
    jobTitle,
    setJobTitle,
    candidateType,
    setCandidateType,
    reviewLoading,
    reviewError,
    review,
    ragReport,
    handleReview,
    scoresBefore,
    scoresAfter,
    annotateError,
    handleDownload,
  }
}
