#!/usr/bin/env node
/**
 * use client な page.tsx を Server シェル + page-content.tsx に分割する。
 * ponytail: 一括変換スクリプト。再実行時は page-content が既にあればスキップ。
 */
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.join(path.dirname(fileURLToPath(import.meta.url)), '..')
const APP_DIR = path.join(ROOT, 'app')

const PUBLIC_PAGES = new Set([
  'privacy/page.tsx',
  'forgot-password/page.tsx',
  'reset-password/page.tsx',
  'verify-email/page.tsx',
  'verify-registration/page.tsx',
  'register/confirm/page.tsx',
  'auth/callback/page.tsx',
])

function findClientPages(dir, acc = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      findClientPages(full, acc)
      continue
    }
    if (entry.name !== 'page.tsx') continue
    const rel = path.relative(APP_DIR, full)
    const src = fs.readFileSync(full, 'utf8')
    if (/^['"]use client['"]/.test(src.trimStart())) acc.push(rel)
  }
  return acc
}

function authMode(rel) {
  if (PUBLIC_PAGES.has(rel)) return 'public'
  if (rel.startsWith('admin/')) return 'admin'
  return 'session'
}

function makeServerPage(mode) {
  if (mode === 'public') {
    return `import PageContent from './page-content'

export default function Page() {
  return <PageContent />
}
`
  }
  if (mode === 'admin') {
    return `import PageContent from './page-content'
import { requireAdminUser } from '@/lib/server-auth'

export default async function Page() {
  await requireAdminUser()
  return <PageContent />
}
`
  }
  return `import PageContent from './page-content'
import { requireSessionUser } from '@/lib/server-auth'

export default async function Page() {
  await requireSessionUser()
  return <PageContent />
}
`
}

function toPageContent(src) {
  let body = src.replace(/^['"]use client['"]\s*\n/, '')
  body = body.replace(/export default function \w+/, 'export default function PageContent')
  return `'use client'\n\n${body.trimStart()}`
}

const pages = findClientPages(APP_DIR)
let converted = 0
let skipped = 0

for (const rel of pages.sort()) {
  const pagePath = path.join(APP_DIR, rel)
  const contentPath = path.join(path.dirname(pagePath), 'page-content.tsx')
  if (fs.existsSync(contentPath)) {
    skipped++
    continue
  }

  const src = fs.readFileSync(pagePath, 'utf8')
  const mode = authMode(rel)
  fs.writeFileSync(contentPath, toPageContent(src))
  fs.writeFileSync(pagePath, makeServerPage(mode))
  converted++
  console.log(`converted: ${rel} (${mode})`)
}

console.log(`done: ${converted} converted, ${skipped} skipped`)
