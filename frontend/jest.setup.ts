import '@testing-library/jest-dom'

// jsdomはResizeObserverを実装していない（ThreeAvatar.tsx等が実装で使用, #1569）
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
}
