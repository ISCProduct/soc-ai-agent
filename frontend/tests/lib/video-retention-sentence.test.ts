import { videoRetentionSentence } from '@/lib/interview/video-retention'

describe('videoRetentionSentence', () => {
  it('学校の日数を文に入れる', () => {
    expect(videoRetentionSentence({ days: 30, unlimited: false }, 'ready')).toContain('30日で削除')
    expect(videoRetentionSentence({ days: null, unlimited: true }, 'ready')).toContain('保管期限を決めていません')
  })

  it('読み込み中と失敗を、日数の断定にしない', () => {
    expect(videoRetentionSentence(null, 'loading')).toContain('確認しています')
    expect(videoRetentionSentence(null, 'error')).toContain('読み込めませんでした')
  })
})