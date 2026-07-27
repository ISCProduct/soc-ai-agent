import React from 'react'

export const metadata = {
  title: 'Admin - AI / RAG Ops',
}

export default function Page() {
  return (
    <div style={{ padding: 24 }}>
      <h1>AI / RAG 運用ダッシュボード</h1>
      <p>ここから RAG キャッシュと LLM コスト運用を管理します。</p>

      <section style={{ marginTop: 24 }}>
        <h2>サマリー</h2>
        <ul>
          <li>RAG コレクション件数: --</li>
          <li>直近更新: --</li>
          <li>キャッシュヒット率: --</li>
          <li>推定月次節約額: --</li>
        </ul>
      </section>

      <section style={{ marginTop: 24 }}>
        <h2>操作</h2>
        <div style={{ display: 'flex', gap: 12 }}>
          <button>キャッシュ再調査（強制）</button>
          <button>コレクション再埋め込み</button>
          <button>キャッシュ優先化を有効化</button>
        </div>
      </section>

      <section style={{ marginTop: 24 }}>
        <h2>ログ / メトリクス</h2>
        <p>ここに短期の RAG 利用ログやヒット率のチャートを表示します（後続実装）</p>
      </section>
    </div>
  )
}
