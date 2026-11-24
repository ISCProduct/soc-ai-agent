'use client'

import type { Dispatch, SetStateAction } from 'react'
import { ListItemIcon, ListItemText, Menu, MenuItem } from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import type { CompanyMenuAnchor } from '../hooks/useCompanyRowActions'

type Props = {
  menuAnchor: CompanyMenuAnchor | null
  setMenuAnchor: Dispatch<SetStateAction<CompanyMenuAnchor | null>>
  isPlatform: boolean
  busy: boolean
  handleFetchPrimary: (companyId: number, force?: boolean) => void
  handlePublish: (companyId: number) => void
  handleReject: (companyId: number) => void
  handleSetVerified: (companyId: number, verified: boolean) => void
}

/** 企業行の「その他の操作」メニュー。中身はすべてシステム管理者専用。 */
export function CompanyActionsMenu({
  menuAnchor,
  setMenuAnchor,
  isPlatform,
  busy,
  handleFetchPrimary,
  handlePublish,
  handleReject,
  handleSetVerified,
}: Props) {
  return (
    <Menu
      anchorEl={menuAnchor?.el}
      open={Boolean(menuAnchor)}
      onClose={() => setMenuAnchor(null)}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      transformOrigin={{ vertical: 'top', horizontal: 'right' }}
    >
      {isPlatform && (
        <MenuItem
          disabled={busy}
          onClick={() => menuAnchor && handleFetchPrimary(menuAnchor.company.id, false)}
        >
          <ListItemIcon>
            <RefreshIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>情報を取得</ListItemText>
        </MenuItem>
      )}
      {isPlatform && (
        <MenuItem
          disabled={busy}
          onClick={() => menuAnchor && handleFetchPrimary(menuAnchor.company.id, true)}
        >
          <ListItemIcon>
            <RefreshIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>最新の情報に更新</ListItemText>
        </MenuItem>
      )}
      {!isPlatform ? null : menuAnchor?.company.data_status !== 'published' ? (
        [
          <MenuItem
            key="publish"
            disabled={busy}
            onClick={() => menuAnchor && handlePublish(menuAnchor.company.id)}
          >
            <ListItemText>学生に公開</ListItemText>
          </MenuItem>,
          <MenuItem
            key="reject"
            disabled={busy}
            onClick={() => menuAnchor && handleReject(menuAnchor.company.id)}
          >
            <ListItemText sx={{ color: 'error.main' }}>公開しない</ListItemText>
          </MenuItem>,
        ]
      ) : (
        <MenuItem disabled={busy} onClick={() => menuAnchor && handleReject(menuAnchor.company.id)}>
          <ListItemText sx={{ color: 'error.main' }}>非公開にする</ListItemText>
        </MenuItem>
      )}
      {isPlatform && menuAnchor ? (
        <MenuItem
          disabled={busy}
          onClick={() =>
            handleSetVerified(menuAnchor.company.id, !menuAnchor.company.is_verified)
          }
        >
          <ListItemText>
            {menuAnchor.company.is_verified ? '学生検索の審査を取り消す' : '学生検索の審査を完了'}
          </ListItemText>
        </MenuItem>
      ) : null}
    </Menu>
  )
}
