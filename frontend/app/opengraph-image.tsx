import { ImageResponse } from 'next/og'

export const alt = '就活AI — IT企業エージェント'
export const size = { width: 1200, height: 630 }
export const contentType = 'image/png'

export default function OpenGraphImage() {
  return new ImageResponse(
    (
      <div
        style={{
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          padding: 80,
          background: '#f8f6f6',
          color: '#0f172a',
        }}
      >
        <div style={{ fontSize: 28, fontWeight: 700, color: '#ec5b13' }}>就活AI</div>
        <div style={{ fontSize: 64, fontWeight: 800, marginTop: 24 }}>IT企業エージェント</div>
        <div style={{ fontSize: 32, color: '#475569', marginTop: 16 }}>
          適性診断から企業マッチングまで
        </div>
      </div>
    ),
    size,
  )
}
