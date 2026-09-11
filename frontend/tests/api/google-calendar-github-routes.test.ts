import { NextRequest } from 'next/server'
import { GET as calendarStatusGET } from '@/app/api/google-calendar/status/route'
import { GET as calendarConnectGET } from '@/app/api/google-calendar/connect/route'
import { DELETE as calendarDisconnectDELETE } from '@/app/api/google-calendar/disconnect/route'
import { GET as githubSkillsGET } from '@/app/api/github/skills/route'
import { GET as githubProfileGET } from '@/app/api/github/profile/route'
import { GET as githubSummariesGET } from '@/app/api/github/repo/summaries/route'
import { POST as githubSummarizePOST } from '@/app/api/github/repo/summarize/route'
import { POST as githubSyncWaitPOST } from '@/app/api/github/sync/wait/route'

// Issue #1013: プロフィールのGoogle Calendar/GitHub連携がブラウザから
// NEXT_PUBLIC_BACKEND_URLを直叩きしていた問題への回帰防止テスト。
// 相対 /api/... 経由でBackendへプロキシされ、X-User-Tokenが転送されることを確認する。
describe('google-calendar / github Next.jsプロキシ', () => {
  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('GET /api/google-calendar/status はX-User-Tokenを転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ connected: true }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest('http://localhost:3000/api/google-calendar/status', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    const response = await calendarStatusGET(request)

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/google-calendar\/status$/),
      expect.objectContaining({ headers: expect.objectContaining({ 'X-User-Token': 'user-jwt' }) }),
    )
    expect(response.status).toBe(200)
  })

  it('GET /api/google-calendar/connect はAccept: application/jsonを付与してauth_urlを中継する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ auth_url: 'https://accounts.google.com/o/oauth2/auth?state=xyz' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    const request = new NextRequest('http://localhost:3000/api/google-calendar/connect', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    const response = await calendarConnectGET(request)
    const data = await response.json()

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/google-calendar\/connect$/),
      expect.objectContaining({
        headers: expect.objectContaining({ 'X-User-Token': 'user-jwt', Accept: 'application/json' }),
      }),
    )
    expect(data.auth_url).toContain('accounts.google.com')
  })

  it('DELETE /api/google-calendar/disconnect はDELETEメソッドで転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ status: 'disconnected' }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest('http://localhost:3000/api/google-calendar/disconnect', {
      method: 'DELETE',
      headers: { 'X-User-Token': 'user-jwt' },
    })
    const response = await calendarDisconnectDELETE(request)

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/google-calendar\/disconnect$/),
      expect.objectContaining({ method: 'DELETE' }),
    )
    expect(response.status).toBe(200)
  })

  it('GET /api/github/skills はBackendのエラーステータスをそのまま返す', async () => {
    jest.spyOn(global, 'fetch').mockResolvedValue(new Response('unauthorized', { status: 401 }))
    const request = new NextRequest('http://localhost:3000/api/github/skills')
    const response = await githubSkillsGET(request)
    expect(response.status).toBe(401)
  })

  it('GET /api/github/profile はBackendへ中継する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ profile: null }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest('http://localhost:3000/api/github/profile', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    await githubProfileGET(request)
    expect(fetchMock).toHaveBeenCalledWith(expect.stringMatching(/\/api\/github\/profile$/), expect.anything())
  })

  it('GET /api/github/repo/summaries はBackendへ中継する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify([]), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest('http://localhost:3000/api/github/repo/summaries', {
      headers: { 'X-User-Token': 'user-jwt' },
    })
    await githubSummariesGET(request)
    expect(fetchMock).toHaveBeenCalledWith(expect.stringMatching(/\/api\/github\/repo\/summaries$/), expect.anything())
  })

  it('POST /api/github/repo/summarize はリクエストボディをそのまま転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ summary: 'ok' }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const body = JSON.stringify({ full_name: 'octocat/hello-world', target_role: 'backend' })
    const request = new NextRequest('http://localhost:3000/api/github/repo/summarize', {
      method: 'POST',
      headers: { 'X-User-Token': 'user-jwt', 'Content-Type': 'application/json' },
      body,
    })
    await githubSummarizePOST(request)

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/github\/repo\/summarize$/),
      expect.objectContaining({ method: 'POST', body }),
    )
  })

  it('POST /api/github/sync/wait はPOSTメソッドで転送する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ status: 'sync completed' }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    )
    const request = new NextRequest('http://localhost:3000/api/github/sync/wait', {
      method: 'POST',
      headers: { 'X-User-Token': 'user-jwt' },
    })
    await githubSyncWaitPOST(request)

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/github\/sync\/wait$/),
      expect.objectContaining({ method: 'POST' }),
    )
  })
})
