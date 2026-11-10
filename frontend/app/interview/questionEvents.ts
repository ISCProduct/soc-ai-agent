export function nextQuestionIndex(currentIndex: number, questionSource: unknown): number {
  return typeof questionSource === 'string' && questionSource.trim() !== ''
    ? currentIndex + 1
    : currentIndex
}
