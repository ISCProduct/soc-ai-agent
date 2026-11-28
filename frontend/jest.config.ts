import type { Config } from 'jest'
import nextJest from 'next/jest.js'

const createJestConfig = nextJest({
  dir: './',
})

const customJestConfig: Config = {
  testEnvironment: 'node',
  setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
  },
  modulePathIgnorePatterns: ['<rootDir>/.next/'],
  testPathIgnorePatterns: ['<rootDir>/e2e/', '<rootDir>/.next/'],
  // カバレッジ（#2046）。テストから読み込まれていないファイルも分母に入れるため、測る範囲を明示する。
  collectCoverageFrom: [
    'app/**/*.{ts,tsx}',
    'components/**/*.{ts,tsx}',
    'lib/**/*.{ts,tsx}',
    '!**/*.d.ts',
    '!**/*.test.{ts,tsx}',
  ],
  // テストの網羅が後退したら CI で落とす（#2046）。--coverage を付けたときだけ効く（手元の --no-coverage では見ない）。
  // 値は 2026-11-26 の develop の実測（文 51.11%・分岐 46.86%・関数 45.14%・行 51.87%）から1〜2ポイント下げた床。
  // テストを足して実測が上がったら、ここも上げる（下げない）。
  coverageThreshold: {
    global: {
      statements: 50,
      branches: 45,
      functions: 44,
      lines: 50,
    },
  },
}

export default createJestConfig(customJestConfig)
