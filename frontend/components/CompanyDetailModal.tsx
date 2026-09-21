"use client"

import { useState } from "react"
import {
  Dialog,
  DialogTitle,
  DialogContent,
  Tabs,
  Tab,
  Card,
  CardContent,
  Button,
  Chip,
  Box,
  Typography,
  IconButton,
} from "@mui/material"
import { X } from "lucide-react"

type CompanyData = {
  name: string
  description: string
  address: string
  employees: number
  tags: string[]
}

type Props = {
  open: boolean
  onCloseAction: () => void
  data: CompanyData
}

export default function CompanyDetailModal({ open, onCloseAction, data }: Props) {
  const [tab, setTab] = useState(0)

  const handleTabChange = (_: any, newValue: number) => {
    setTab(newValue)
  }

  return (
      <Dialog open={open} onClose={onCloseAction} maxWidth="md" fullWidth>
        {/* Header */}
        <Box display="flex" justifyContent="space-between" alignItems="center" px={2} py={1}>
          <DialogTitle sx={{ m: 0, p: 0 }}>{data.name}</DialogTitle>
          <IconButton onClick={onCloseAction}>
            <X size={22} />
          </IconButton>
        </Box>

        <DialogContent dividers sx={{ pt: 2 }}>
          {/* Tabs */}
          <Tabs value={tab} onChange={handleTabChange}>
            <Tab label="基本情報" />
            <Tab label="詳細" />
          </Tabs>

          {/* Tab Panels */}
          {tab === 0 && (
              <Box mt={2}>
                <Card variant="outlined">
                  <CardContent>
                    <Typography variant="body1" gutterBottom>
                      {data.description}
                    </Typography>

                    <Typography variant="subtitle2" mt={2}>
                      住所
                    </Typography>
                    <Typography variant="body2">{data.address}</Typography>

                    <Typography variant="subtitle2" mt={2}>
                      従業員数
                    </Typography>
                    <Typography variant="body2">{data.employees} 名</Typography>

                    <Typography variant="subtitle2" mt={2}>
                      タグ
                    </Typography>
                    <Box mt={1} display="flex" gap={1} flexWrap="wrap">
                      {data.tags.map((tag) => (
                          <Chip key={tag} label={tag} variant="outlined" />
                      ))}
                    </Box>
                  </CardContent>
                </Card>
              </Box>
          )}

          {tab === 1 && (
              <Box mt={2}>
                <Card variant="outlined">
                  <CardContent>
                    <Typography variant="body1">追加の詳細情報をここに表示</Typography>
                  </CardContent>
                </Card>
              </Box>
          )}

          {/* Actions */}
          <Box mt={3} textAlign="right">
            <Button variant="contained" color="primary" onClick={onCloseAction}>
              閉じる
            </Button>
          </Box>
        </DialogContent>
      </Dialog>
  )
}
