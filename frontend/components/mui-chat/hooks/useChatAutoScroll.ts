'use client'

import { useEffect, useRef } from 'react'
import useMediaQuery from '@mui/material/useMediaQuery'
import { shouldAutoScrollToBottom } from '../utils'
import type { Message } from '../types'

/**
 * メッセージ一覧の自動スクロール。
 *
 * 一覧の領域と末尾の目印の ref を持ち、やり取りが増えたときだけ末尾へ送る。
 * 利用者が上へ遡っているあいだは追わない（shouldAutoScrollToBottom）。
 *
 * 動きを減らす設定（prefers-reduced-motion）では滑らせずに移動する。
 * 返答1件ごとに画面が流れるのは、前庭障害や光過敏のある利用者には
 * 繰り返し動く要素と同じ負荷になる。
 * ただし移動そのものは止めない。止めると最新の発言が画面外に残り、
 * 動きを消す代わりに情報を失う。
 */
export function useChatAutoScroll(messages: Message[], isLoading: boolean) {
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const messagesAreaRef = useRef<HTMLDivElement>(null)
  // 既存の図版（CompanyDiagram など）と同じ MUI の判定を使う。
  const reduceMotion = useMediaQuery('(prefers-reduced-motion: reduce)')

  useEffect(() => {
    const area = messagesAreaRef.current
    if (area) {
      const allow = shouldAutoScrollToBottom({
        scrollHeight: area.scrollHeight,
        scrollTop: area.scrollTop,
        clientHeight: area.clientHeight,
      })
      if (!allow) return
    }
    messagesEndRef.current?.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth' })
  }, [messages, isLoading, reduceMotion])

  return { messagesEndRef, messagesAreaRef }
}
