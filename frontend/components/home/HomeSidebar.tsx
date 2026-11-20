'use client'

import { useEffect, useState, type ReactNode } from 'react'
import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { Avatar, Box, Button, Drawer, Tooltip, Typography, type SvgIconProps } from '@mui/material'
import AdminPanelSettings from '@mui/icons-material/AdminPanelSettings'
import Assessment from '@mui/icons-material/Assessment'
import Assignment from '@mui/icons-material/Assignment'
import BorderAll from '@mui/icons-material/BorderAll'
import Business from '@mui/icons-material/Business'
import CalendarMonth from '@mui/icons-material/CalendarMonth'
import Description from '@mui/icons-material/Description'
import EditNote from '@mui/icons-material/EditNote'
import History from '@mui/icons-material/History'
import Home from '@mui/icons-material/Home'
import MailOutline from '@mui/icons-material/MailOutline'
import ManageAccounts from '@mui/icons-material/ManageAccounts'
import Quiz from '@mui/icons-material/Quiz'
import RecordVoiceOver from '@mui/icons-material/RecordVoiceOver'
import Search from '@mui/icons-material/Search'
import { authService, type User } from '@/lib/auth'
import { HOME_COLORS, HOME_LAYOUT } from '@/lib/home-theme'
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
  '/applications': Assignment,
  '/companies': Search,
  '/scout': MailOutline,
  '/profile': ManageAccounts,
  '/admin': AdminPanelSettings,
}

const headingSx = { fontWeight: 700, fontSize: 16, lineHeight: 1.5, color: HOME_COLORS.ink } as const

/**
 * ホーム（相談中心）の左サイドバー（Figma「改善G／相談中心」ノード 37:340）。
 * ユーザー、ホーム・マッチング結果、自己分析の進捗、準備する・就活を進めるの2グループ、プロフィール設定。
 * 現在地は、色・左のバー・太字で示す（色だけで伝えない）。
 */
export function HomeSidebar({ user, onLogout, mobileOpen = false, onMobileClose }: HomeSidebarProps) {
  const router = useRouter()
  const pathname = usePathname() ?? '/'
  // ナビの表示名は辞書（messages/ja.json の nav）が正本
  const t = useTranslations('nav')
  const [questionCount, setQuestionCount] = useState(0)
  const [totalQuestions, setTotalQuestions] = useState(15)
  const [judgmentPercent, setJudgmentPercent] = useState<number | null>(null)
  const [phases, setPhases] = useState<PhaseProgress[] | null>(null)
  const [isAdmin, setIsAdmin] = useState(!!user.is_admin)

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
          px: 1.5,
          borderRadius: '4px',
          textDecoration: 'none',
          bgcolor: current ? HOME_COLORS.activeBg : 'transparent',
          color: current ? HOME_COLORS.primary : HOME_COLORS.navInk,
          fontWeight: current ? 700 : 400,
          fontSize: 16,
          lineHeight: 1.5,
          '&:hover': { bgcolor: current ? HOME_COLORS.activeBg : HOME_COLORS.page },
        }}
      >
        {/* 現在地のバー（色に加えて形でも示す） */}
        <Box aria-hidden sx={{ width: 4, height: 24, flexShrink: 0, bgcolor: current ? HOME_COLORS.primary : 'transparent' }} />
        {Icon && <Icon aria-hidden fontSize="small" sx={{ flexShrink: 0, color: current ? HOME_COLORS.primary : HOME_COLORS.link }} />}
        <Box sx={{ flex: 1, minWidth: 0 }}>
          {label}
          {opts.secondary && (
            <Typography component="span" sx={{ display: 'block', fontSize: 12, color: HOME_COLORS.soft, fontWeight: 400 }}>
              {opts.secondary}
            </Typography>
          )}
        </Box>
      </Box>
    )
  }

  const content = (
    <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100%', p: 2, bgcolor: HOME_COLORS.white }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 1.5, gap: 1 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
          <Avatar sx={{ width: 32, height: 32, bgcolor: HOME_COLORS.link, fontSize: 16 }}>{user.name.charAt(0).toUpperCase()}</Avatar>
          <Typography noWrap sx={{ ...headingSx }}>
            {user.name}
          </Typography>
          {user.is_guest && (
            <Typography component="span" sx={{ fontSize: 12, color: HOME_COLORS.soft }}>
              ゲスト
            </Typography>
          )}
        </Box>
        <Button
          onClick={onLogout}
          sx={{ minWidth: 0, p: 0.5, fontSize: 16, fontWeight: 400, color: HOME_COLORS.soft, textTransform: 'none', flexShrink: 0 }}
        >
          ログアウト
        </Button>
      </Box>
      <Box sx={{ height: 8 }} />

      <Box component="nav" aria-label="メニュー" sx={{ display: 'flex', flexDirection: 'column', flex: 1 }}>
        {navItem(HOME_SIDEBAR_TOP[0].href, HOME_SIDEBAR_TOP[0].label)}
        {navItem(HOME_SIDEBAR_TOP[1].href, HOME_SIDEBAR_TOP[1].label, {
          // 結果画面へは、いまのチャットの session を付けて遷移する（付けられないときはチャットへ）
          onClick: (e) => {
            e.preventDefault()
            router.push(getResultsPathOrChat())
          },
        })}

        <Box sx={{ height: 12 }} />
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 1.5, fontSize: 16, lineHeight: 1.5 }}>
          <Typography component="span" sx={headingSx}>
            {t('groups.analysis')}
          </Typography>
          <Typography component="span" sx={{ fontSize: 16, color: HOME_COLORS.soft }}>
            {`判定の進み具合 ${progress.percent}%`}
          </Typography>
        </Box>
        <Box sx={{ px: 1.5, py: 0.5 }}>
          <Box
            role="progressbar"
            aria-label="自己分析の進み具合"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={progress.percent}
            sx={{ height: 8, borderRadius: '4px', bgcolor: HOME_COLORS.track, overflow: 'hidden' }}
          >
            <Box sx={{ height: '100%', width: `${progress.percent}%`, bgcolor: HOME_COLORS.primary }} />
          </Box>
        </Box>
        {HOME_SIDEBAR_ANALYSIS.map((i) => navItem(i.href, i.label))}

        {HOME_SIDEBAR_GROUPS.map((group) => (
          <Box key={group.heading} sx={{ display: 'flex', flexDirection: 'column' }}>
            <Box sx={{ height: 12 }} />
            <Typography component="p" sx={{ ...headingSx, px: 1.5 }}>
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

        <Box sx={{ flex: 1, minHeight: 16 }} />
        {navItem(HOME_SIDEBAR_FOOTER.href, HOME_SIDEBAR_FOOTER.label)}
        {isAdmin && navItem(SIDEBAR_ADMIN_NAV.href, SIDEBAR_ADMIN_NAV.label)}
      </Box>
    </Box>
  )

  const drawerSx = {
    width: HOME_LAYOUT.sidebarWidth,
    flexShrink: 0,
    '& .MuiDrawer-paper': {
      width: HOME_LAYOUT.sidebarWidth,
      boxSizing: 'border-box',
      bgcolor: HOME_COLORS.white,
      borderRight: `1px solid ${HOME_COLORS.sidebarBorder}`,
      overflowY: 'auto',
    },
  }

  return (
    <>
      {/* モバイル用: Temporary Drawer */}
      <Drawer variant="temporary" open={mobileOpen} onClose={onMobileClose} ModalProps={{ keepMounted: true }} className={styles.drawerMobile} sx={drawerSx}>
        {content}
      </Drawer>
      {/* デスクトップ用: Permanent Drawer */}
      <Drawer variant="permanent" className={styles.drawerDesktop} sx={drawerSx} open>
        {content}
      </Drawer>
    </>
  )
}
