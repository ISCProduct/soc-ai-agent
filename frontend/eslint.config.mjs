import tsParser from '@typescript-eslint/parser'
import tsPlugin from '@typescript-eslint/eslint-plugin'
import reactHooksPlugin from 'eslint-plugin-react-hooks'

export default [
  {
    // vendor/live2d-framework は Live2D公式 Cubism Web Framework の無改変ベンダリング。
    // 自分たちのコードではないのでlintしない（strict向けでない上流コードなので警告まみれになる）。
    ignores: ['.next/**', 'node_modules/**', 'vendor/live2d-framework/**'],
  },
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsParser,
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
      'react-hooks': reactHooksPlugin,
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'warn',
      // 早期 return より後ろのフックは「Rendered more hooks than during the previous render」で
      // 画面ごと落ちる。プラグインは入っていたのにルール未設定で素通りしていた
      'react-hooks/rules-of-hooks': 'error',
    },
  },
]
