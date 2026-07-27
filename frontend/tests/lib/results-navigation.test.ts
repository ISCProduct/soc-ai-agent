/** @jest-environment jsdom */

import {
  getStoredChatSessionId,
  getResultsSessionContext,
  buildResultsPath,
  getResultsPathOrChat,
} from '@/lib/results-navigation'
import { authService } from '@/lib/auth'

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: jest.fn(),
  },
}))

const mockGetStoredUser = authService.getStoredUser as jest.MockedFunction<
  typeof authService.getStoredUser
>

function createStorageMock(): Storage {
  const store = new Map<string, string>()
  return {
    get length() {
      return store.size
    },
    clear: () => store.clear(),
    getItem: (key: string) => store.get(key) ?? null,
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key)
    },
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
  }
}

describe('results-navigation', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'sessionStorage', {
      value: createStorageMock(),
      configurable: true,
    })
    Object.defineProperty(window, 'localStorage', {
      value: createStorageMock(),
      configurable: true,
    })
    mockGetStoredUser.mockReturnValue(null)
  })

  it('getStoredChatSessionId prefers sessionStorage', () => {
    sessionStorage.setItem('chatSessionId', 'sess-1')
    localStorage.setItem('currentSessionId', 'sess-2')
    expect(getStoredChatSessionId()).toBe('sess-1')
  })

  it('getStoredChatSessionId falls back to localStorage', () => {
    localStorage.setItem('currentSessionId', 'sess-2')
    expect(getStoredChatSessionId()).toBe('sess-2')
  })

  it('getResultsSessionContext returns null when user or session missing', () => {
    expect(getResultsSessionContext()).toBeNull()

    mockGetStoredUser.mockReturnValue({
      user_id: 42,
      email: 'a@b.c',
      name: 'Test',
      is_guest: false,
    })
    expect(getResultsSessionContext()).toBeNull()
  })

  it('getResultsSessionContext returns context when both present', () => {
    mockGetStoredUser.mockReturnValue({
      user_id: 42,
      email: 'a@b.c',
      name: 'Test',
      is_guest: false,
    })
    sessionStorage.setItem('chatSessionId', 'sess-99')
    expect(getResultsSessionContext()).toEqual({ userId: '42', sessionId: 'sess-99' })
  })

  it('buildResultsPath encodes query params', () => {
    expect(buildResultsPath({ userId: '42', sessionId: 'sess-99' })).toBe(
      '/results?user_id=42&session_id=sess-99'
    )
  })

  it('getResultsPathOrChat falls back to chat', () => {
    expect(getResultsPathOrChat()).toBe('/')
  })

  it('getResultsPathOrChat returns results path when session exists', () => {
    mockGetStoredUser.mockReturnValue({
      user_id: 7,
      email: 'a@b.c',
      name: 'Test',
      is_guest: false,
    })
    sessionStorage.setItem('chatSessionId', 'abc')
    expect(getResultsPathOrChat()).toBe('/results?user_id=7&session_id=abc')
  })
})
