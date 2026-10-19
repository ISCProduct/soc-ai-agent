'use client'

import { useSyncExternalStore } from 'react'

/**
 * AI面接のセッション画面（SessionScreen）が表示中かどうかを、
 * ページ全体に固定表示される StudentBottomNav へ伝えるための最小限の共有フラグ（#1569）。
 *
 * SessionScreen・SelectionScreen・LobbyScreen は全て同一パス `/interview` 上で
 * 内部 status を切り替えて表示される（ルーティングしない）ため、
 * `shouldShowStudentBottomNav(pathname)` のようなパス判定ではセッション中だけを
 * 区別できない。SessionScreen は常に 100dvh 固定の全画面レイアウトで、
 * 下部ナビ（position: fixed）が「話す」ボタン等に重なってしまうため、
 * セッション画面がマウントされている間だけ下部ナビを非表示にする。
 */
let interviewSessionActive = false
const listeners = new Set<() => void>()

export function setInterviewSessionActive(active: boolean): void {
  if (interviewSessionActive === active) return
  interviewSessionActive = active
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function getSnapshot(): boolean {
  return interviewSessionActive
}

function getServerSnapshot(): boolean {
  return false
}

export function useInterviewSessionActive(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot)
}
