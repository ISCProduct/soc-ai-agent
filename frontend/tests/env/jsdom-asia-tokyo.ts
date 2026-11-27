import JSDOMEnvironment from 'jest-environment-jsdom'

/**
 * タイムゾーンを日本時間（Asia/Tokyo）に固定した jsdom のテスト環境（#2092）。
 *
 * 画面が日時を実行環境のタイムゾーン（利用者の端末＝日本時間）で扱うテストのためのもの。
 * テストの中で `process.env.TZ` を書き換えても、Jest のテストの中の `process.env` は写しなので
 * ワーカーの Date には効かない（CI の runner は UTC）。テスト環境はワーカーの本物の process で動くため、
 * ここで TZ を変えると、テストファイルの読み込みより前に Date のタイムゾーンが日本時間になる。
 * 同じワーカーで次に動くテストファイルに持ち越さないよう、片付けで元に戻す。
 *
 * 使い方: テストファイルの先頭に `@jest-environment ./tests/env/jsdom-asia-tokyo.ts` を書く。
 */
export default class JsdomAsiaTokyoEnvironment extends JSDOMEnvironment {
  private originalTZ: string | undefined

  async setup(): Promise<void> {
    this.originalTZ = process.env.TZ
    process.env.TZ = 'Asia/Tokyo'
    await super.setup()
  }

  async teardown(): Promise<void> {
    if (this.originalTZ === undefined) {
      delete process.env.TZ
    } else {
      process.env.TZ = this.originalTZ
    }
    await super.teardown()
  }
}
