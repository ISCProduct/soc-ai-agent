'use client'

import { useEffect, useState, type ReactNode } from 'react'
import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { Avatar, Box, Button, Drawer, IconButton, Tooltip, Typography, type SvgIconProps } from '@mui/material'
import AdminPanelSettings from '@mui/icons-material/AdminPanelSettings'
import Assessment from '@mui/icons-material/Assessment'
import Assignment from '@mui/icons-material/Assignment'
import BorderAll from '@mui/icons-material/BorderAll'
import Business from '@mui/icons-material/Business'
import CalendarMonth from '@mui/icons-material/CalendarMonth'
import Close from '@mui/icons-material/Close'
import Description from '@mui/icons-material/Description'
import EditNote from '@mui/icons-material/EditNote'
import Favorite from '@mui/icons-material/Favorite'
import History from '@mui/icons-material/History'
import Home from '@mui/icons-material/Home'
import MailOutline from '@mui/icons-material/MailOutline'
import ManageAccounts from '@mui/icons-material/ManageAccounts'
import Quiz from '@mui/icons-material/Quiz'
import RecordVoiceOver from '@mui/icons-material/RecordVoiceOver'
import Search from '@mui/icons-material/Search'
import { authService, type User } from '@/lib/auth'
import { HOME_COLORS, HOME_LAYOUT } from '@/lib/home-theme'
import { STUDENT_FOCUS_OUTLINE, UI } from '@/lib/design/tokens'
import { useStudentTheme } from '@/components/providers/MuiProvider'
import { getResultsPathOrChat } from '@/lib/results-navigation'
import { getApplicationsNavTarget } from '@/lib/sidebar-navigation'
import {
  HOME_SIDEBAR_ANALYSIS,
  HOME_SIDEBAR_FOOTER,
  HOME_SIDEBAR_GROUPS,
  HOME_SIDEBAR_TOP,
  SIDEBAR_ADMIN_NAV,
  isCurrentNavExclusive,
} from '@/lib/sidebar-nav'
import { useTranslations } from '@/lib/i18n'
import { computeProgressTotals } from '../mui-chat/utils'
import styles from './HomeSidebar.module.css'

interface PhaseProgress {
  questions_asked?: number
  valid_answers?: number
  min_questions?: number
  max_questions?: number
}

interface HomeSidebarProps {
  user: User
  onLogout: () => void
  mobileOpen?: boolean
  onMobileClose?: () => void
}

const ALL_HREFS: readonly string[] = [
  ...HOME_SIDEBAR_TOP.map((i) => i.href),
  ...HOME_SIDEBAR_ANALYSIS.map((i) => i.href),
  ...HOME_SIDEBAR_GROUPS.flatMap((g) => g.items.map((i) => i.href)),
  HOME_SIDEBAR_FOOTER.href,
]

/**
 * 項目ごとのアイコン。旧サイドバー（AnalysisSidebar）で使っていたものに合わせる（ホームだけは新設）。
 * 文字が正本で、アイコンは見つけやすくするための補助（読み上げには出さない）。
 */
const NAV_ICONS: Record<string, React.ComponentType<SvgIconProps>> = {
  '/': Home,
  '/results': Business,
  '/correlation-diagram': BorderAll,
  '/chat-history': History,
  '/resume': Description,
  '/es-rewrite': EditNote,
  '/assessments': Quiz,
  '/ability-assessments': Quiz,
  '/interview': RecordVoiceOver,
  '/interview/history': Assessment,
  '/schedule': CalendarMonth,
  '/favorites': Favorite,
  '/applications': Assignment,
  '/companies': Search,
  '/scout': MailOutline,
  '/profile': ManageAccounts,
  '/admin': AdminPanelSettings,
}

const sectionHeadingSx = { fontWeight: 600, fontSize: 13, lineHeight: 1.5, color: UI.inkSoft } as const
const hiddenScrollbarSx = {
  scrollbarWidth: 'none',
  '&::-webkit-scrollbar': { display: 'none' },
} as const

/** スクロールバーを隠していても、下に続きがあることを伝える。 */
function useSidebarScrollHint(enabled = true) {
  // モバイルの Portal は後からマウントされるため、要素が揃ってから監視を始める。
  const [viewport, viewportRef] = useState<HTMLElement | null>(null)
  const [content, contentRef] = useState<HTMLDivElement | null>(null)
  const [hasMoreBelow, setHasMoreBelow] = useState(false)

  useEffect(() => {
    if (!enabled || !viewport || !content) return

    const update = () => {
      // 小数点を含むスクロール位置と、非表示のドロワーを考慮する。
      setHasMoreBelow(viewport.clientHeight > 0 && viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight > 1)
    }
    update()
    const observer = new ResizeObserver(update)
    observer.observe(viewport)
    observer.observe(content)
    viewport.addEventListener('scroll', update, { passive: true })
    window.addEventListener('resize', update)
    return () => {
      observer.disconnect()
      viewport.removeEventListener('scroll', update)
      window.removeEventListener('resize', update)
    }
  }, [enabled, viewport, content])

  return { viewportRef, contentRef, hasMoreBelow: enabled && hasMoreBelow }
}

function ScrollHint({ visible }: { visible: boolean }) {
  if (!visible) return null
  return (
    <Box
      data-testid="menu-scroll-fade"
      aria-hidden="true"
      sx={{
        position: 'absolute', bottom: 0, left: 0, right: 0, height: 24, zIndex: 1,
        pointerEvents: 'none',
        background: `linear-gradient(to bottom, transparent, ${UI.field})`,
      }}
    />
  )
}

/**
 * ホーム（相談中心）の左サイドバー（Figma「改善G／相談中心」ノード 37:340）。
 * ユーザー、ホーム・マッチング結果、自己分析の進捗、準備する・就活を進めるの2グループ、プロフィール設定。
 * 現在地は、色・左のバー・太字で示す（色だけで伝えない）。
 */
export function HomeSidebar({ user, onLogout, mobileOpen = false, onMobileClose }: HomeSidebarProps) {
  const router = useRouter()
  const pathname = usePathname() ?? '/'
  const { mode } = useStudentTheme()
  // 通常表示は既存ホームの青を保ち、高コントラスト時はテーマの青に追従する。
  const navIconColor = mode === 'high-contrast' ? UI.mark : HOME_COLORS.link
  // ナビの表示名は辞書（messages/ja.json の nav）が正本
  const t = useTranslations('nav')
  const [questionCount, setQuestionCount] = useState(0)
  const [totalQuestions, setTotalQuestions] = useState(15)
  const [judgmentPercent, setJudgmentPercent] = useState<number | null>(null)
  const [phases, setPhases] = useState<PhaseProgress[] | null>(null)
  const [isAdmin, setIsAdmin] = useState(!!user.is_admin)
  const mobileScroll = useSidebarScrollHint(mobileOpen)
  const desktopScroll = useSidebarScrollHint()

  useEffect(() => {
    setIsAdmin(!!user.is_admin)
    const refreshAdminFlag = async () => {
      if (!user?.user_id || user.is_admin) return
      try {
        const fresh = await authService.getUser()
        if (fresh?.is_admin) {
          setIsAdmin(true)
          await authService.saveAuth({
            user_id: fresh.user_id,
            email: fresh.email,
            name: fresh.name,
            is_guest: fresh.is_guest,
            target_level: fresh.target_level,
            school_name: fresh.school_name,
            is_admin: fresh.is_admin,
            is_staff: fresh.is_staff,
            certifications_acquired: fresh.certifications_acquired,
            certifications_in_progress: fresh.certifications_in_progress,
            oauth_provider: fresh.oauth_provider,
            avatar_url: fresh.avatar_url,
          })
        }
      } catch {
        // 管理者かどうかの再確認に失敗しても、ナビは使える
      }
    }
    void refreshAdminFlag()

    // 診断の進捗はチャット（useMuiChat）が chatProgress で知らせる
    const handleChatProgress = (event: CustomEvent) => {
      setQuestionCount(event.detail.questionCount || 0)
      setTotalQuestions(event.detail.totalQuestions || 15)
      setJudgmentPercent(typeof event.detail.judgmentPercent === 'number' ? event.detail.judgmentPercent : null)
      if (event.detail.phases) setPhases(event.detail.phases as PhaseProgress[])
    }
    window.addEventListener('chatProgress', handleChatProgress as EventListener)
    return () => window.removeEventListener('chatProgress', handleChatProgress as EventListener)
  }, [user])

  const progress = computeProgressTotals({ phases, questionCount, totalQuestions, judgmentPercent })

  const navItem = (
    href: string,
    label: string,
    opts: { onClick?: (e: React.MouseEvent) => void; secondary?: string; hrefOverride?: string } = {},
  ): ReactNode => {
    const current = isCurrentNavExclusive(pathname, href, ALL_HREFS)
    const Icon = NAV_ICONS[href]
    return (
      <Box
        key={href}
        component={Link}
        href={opts.hrefOverride ?? href}
        prefetch={false}
        aria-current={current ? 'page' : undefined}
        onClick={(e: React.MouseEvent) => {
          opts.onClick?.(e)
          onMobileClose?.()
        }}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1.5,
          minHeight: 44,
          flexShrink: 0,
          px: 1.5,
          borderRadius: 1,
          textDecoration: 'none',
          bgcolor: current ? UI.markTint : 'transparent',
          color: current ? UI.mark : UI.ink,
          typography: 'body1',
          fontWeight: current ? 700 : 400,
          '&:hover': { bgcolor: current ? UI.markTint : UI.tint },
          '&:focus-visible': { outline: STUDENT_FOCUS_OUTLINE, outlineOffset: -2 },
        }}
      >
        {/* 現在地のバー（色に加えて形でも示す） */}
        <Box aria-hidden sx={{ width: 4, height: 24, flexShrink: 0, borderRadius: '2px', bgcolor: current ? UI.mark : 'transparent' }} />
        {Icon && <Icon aria-hidden fontSize="small" sx={{ flexShrink: 0, color: current ? UI.mark : navIconColor }} />}
        <Box sx={{ flex: 1, minWidth: 0 }}>
          {label}
          {opts.secondary && (
            <Typography component="span" sx={{ display: 'block', fontSize: 12, color: UI.inkSoft, fontWeight: 400 }}>
              {opts.secondary}
            </Typography>
          )}
        </Box>
      </Box>
    )
  }

  const content = (mobile: boolean) => (
    <Box ref={mobile ? undefined : desktopScroll.contentRef} sx={{ display: 'flex', flexDirection: 'column', height: { xs: '100%', md: 'auto' }, minHeight: '100%', p: { xs: 0, md: 2 }, bgcolor: UI.field }}>
      {/* モバイルでは上部をスクロール領域の外に置き、いつでも閉じられるようにする。 */}
      <Box sx={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexShrink: 0,
        px: { xs: 2, md: 1.5 }, gap: 1,
        pt: { xs: 'calc(8px + env(safe-area-inset-top))', md: 0 },
        pb: { xs: 1, md: 0 },
        borderBottom: { xs: `1px solid ${UI.ruleSoft}`, md: 'none' },
      }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
          <Avatar sx={{ width: 32, height: 32, bgcolor: UI.markTint, color: UI.mark, fontSize: 16 }}>{user.name.charAt(0).toUpperCase()}</Avatar>
          <Typography noWrap sx={{ fontWeight: 600, fontSize: 14, lineHeight: 1.5, color: UI.ink }}>
            {user.name}
          </Typography>
          {user.is_guest && (
            <Typography component="span" sx={{ fontSize: 12, color: UI.inkSoft }}>
              ゲスト
            </Typography>
          )}
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}>
          <Button
            onClick={onLogout}
            sx={{
              minWidth: 0, p: 0.5, fontSize: 12, fontWeight: 400, color: UI.inkSoft, textTransform: 'none', flexShrink: 0,
              '&:hover': { bgcolor: UI.tint },
              '&:focus-visible': { outline: STUDENT_FOCUS_OUTLINE, outlineOffset: -2 },
            }}
          >
            ログアウト
          </Button>
          {mobile && (
            <IconButton
              aria-label={t('menu.close')}
              onClick={onMobileClose}
              sx={{
                width: 44, height: 44, flexShrink: 0, color: UI.inkSoft,
                '&:hover': { bgcolor: UI.tint },
                '&:focus-visible': { outline: STUDENT_FOCUS_OUTLINE, outlineOffset: -2 },
              }}
            >
              <Close />
            </IconButton>
          )}
        </Box>
      </Box>
      <Box sx={{ height: 8, flexShrink: 0 }} />

      <Box ref={mobile ? mobileScroll.viewportRef : undefined} component="nav" aria-label="メニュー" sx={{
        display: 'flex', flexDirection: 'column', flex: 1,
        minHeight: { xs: 0, md: 'auto' },
        overflowY: { xs: 'auto', md: 'visible' },
        scrollPaddingBottom: 24,
        overscrollBehaviorY: { xs: 'contain', md: 'auto' },
        px: { xs: 2, md: 0 },
        // 末尾の項目と画面端の間に32pxを確保（PCは外側の16pxと合わせる）。
        pb: { xs: 'calc(32px + env(safe-area-inset-bottom))', md: 2 },
        ...hiddenScrollbarSx,
      }}>
        <Box ref={mobile ? mobileScroll.contentRef : undefined} sx={{ display: 'flex', flexDirection: 'column', flex: '1 0 auto' }}>
          {navItem(HOME_SIDEBAR_TOP[0].href, HOME_SIDEBAR_TOP[0].label)}
          {navItem(HOME_SIDEBAR_TOP[1].href, HOME_SIDEBAR_TOP[1].label, {
            // 結果画面へは、いまのチャットの session を付けて遷移する（付けられないときはチャットへ）
            onClick: (e) => {
              e.preventDefault()
              router.push(getResultsPathOrChat())
            },
          })}

          <Box sx={{ height: 24, flexShrink: 0 }} />
          <Box sx={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 1, minHeight: 24, flexShrink: 0, px: 1.5 }}>
            <Typography component="span" sx={sectionHeadingSx}>
              {t('groups.analysis')}
            </Typography>
            <Typography
              component="span"
              sx={{ fontSize: 12, lineHeight: 1.5, fontWeight: 400, color: UI.inkSoft, whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' }}
            >
              {`判定の進み具合 ${progress.percent}%`}
            </Typography>
          </Box>
          <Box sx={{ px: 1.5, py: 0.5, flexShrink: 0 }}>
            <Box
              role="progressbar"
              aria-label="自己分析の進み具合"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={progress.percent}
              sx={{ height: 8, borderRadius: '4px', bgcolor: UI.ruleSoft, overflow: 'hidden' }}
            >
              <Box sx={{ height: '100%', width: `${progress.percent}%`, bgcolor: UI.mark }} />
            </Box>
          </Box>
          {HOME_SIDEBAR_ANALYSIS.map((i) => navItem(i.href, i.label))}

          {HOME_SIDEBAR_GROUPS.map((group) => (
            <Box key={group.heading} sx={{ display: 'flex', flexDirection: 'column', flexShrink: 0 }}>
              <Box sx={{ height: 24, flexShrink: 0 }} />
              <Typography component="p" sx={{ ...sectionHeadingSx, px: 1.5 }}>
                {group.heading}
              </Typography>
              {group.items.map((i) =>
                i.href === '/applications' ? (
                  // ゲストは選考管理を使えない。押すとログインへ送る（既存の挙動）
                  <Tooltip key={i.href} title={user.is_guest ? '選考管理はログイン後に利用できます' : ''} disableHoverListener={!user.is_guest}>
                    <Box>
                      {navItem(i.href, i.label, {
                        hrefOverride: getApplicationsNavTarget(!!user.is_guest),
                        secondary: user.is_guest ? 'ログインが必要です' : undefined,
                      })}
                    </Box>
                  </Tooltip>
                ) : (
                  navItem(i.href, i.label)
                ),
              )}
            </Box>
          ))}

          <Box sx={{ flex: 1, minHeight: 24 }} />
          {navItem(HOME_SIDEBAR_FOOTER.href, HOME_SIDEBAR_FOOTER.label)}
          {isAdmin && navItem(SIDEBAR_ADMIN_NAV.href, SIDEBAR_ADMIN_NAV.label)}
        </Box>
      </Box>
    </Box>
  )

  const drawerSx = {
    width: { xs: 'min(360px, calc(100vw - 24px))', md: HOME_LAYOUT.sidebarWidth },
    flexShrink: 0,
    '& .MuiDrawer-paper': {
      width: { xs: 'min(360px, calc(100vw - 24px))', md: HOME_LAYOUT.sidebarWidth },
      height: { xs: '100dvh', md: '100%' },
      boxSizing: 'border-box',
      bgcolor: UI.field,
      boxShadow: 'none',
      borderRight: `1px solid ${UI.ruleSoft}`,
      // フェードはスクロール領域の外に重ね、常に画面の下端に置く。
      overflowY: 'hidden',
    },
  }

  return (
    <>
      {/* モバイル用: Temporary Drawer */}
      <Drawer variant="temporary" open={mobileOpen} onClose={onMobileClose} ModalProps={{ keepMounted: true }} className={styles.drawerMobile} sx={drawerSx}>
        {content(true)}
        <ScrollHint visible={mobileScroll.hasMoreBelow} />
      </Drawer>
      {/* デスクトップ用: Permanent Drawer */}
      <Drawer variant="permanent" className={styles.drawerDesktop} sx={drawerSx} open>
        <Box
          ref={desktopScroll.viewportRef}
          data-testid="desktop-menu-scroll"
          sx={{ height: '100%', minHeight: 0, overflowY: 'auto', scrollPaddingBottom: 24, ...hiddenScrollbarSx }}
        >
          {content(false)}
        </Box>
        <ScrollHint visible={desktopScroll.hasMoreBelow} />
      </Drawer>
    </>
  )
}
