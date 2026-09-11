import '@/styles/globals.css'

// このページのみ shadcn/ui (Tailwind) を使用するため、Tailwind の CSS をこのサブツリーに限定して読み込む。
// 他の管理画面は引き続き MUI を使用する。
export default function AdminCostsLayout({ children }: { children: React.ReactNode }) {
  return children
}
