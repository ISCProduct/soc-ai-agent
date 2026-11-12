import { TextDecoder, TextEncoder } from 'util'
import { parseMultipartResponse } from '@/lib/interview/utils'

Object.assign(globalThis, { TextEncoder, TextDecoder })

/** 面接ターンの応答（multipart/mixed: JSONメタ + 音声）を実際の形で組み立てる */
function turnResponse(audioType: string, audio: string): Response {
  const boundary = 'turnboundary'
  const body =
    `--${boundary}\r\nContent-Type: application/json\r\n\r\n${JSON.stringify({ ai_text: '次の質問です', ai_fallback: ['tts'] })}\r\n` +
    `--${boundary}\r\nContent-Type: ${audioType}\r\n\r\n${audio}\r\n` +
    `--${boundary}--\r\n`
  const bytes = new TextEncoder().encode(body)
  return {
    headers: { get: () => `multipart/mixed; boundary=${boundary}` },
    arrayBuffer: async () => bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength),
  } as unknown as Response
}

describe('parseMultipartResponse', () => {
  it('OpenAI 障害時のローカル TTS（WAV）は audio/wav の Blob にする（#1833）', async () => {
    const { meta, audio } = await parseMultipartResponse(turnResponse('audio/wav', 'RIFFxxxxWAVE'))
    expect(audio.type).toBe('audio/wav')
    expect(meta.ai_text).toBe('次の質問です')
    expect(meta.ai_fallback).toEqual(['tts'])
  })

  it('OpenAI の TTS（mp3）は今まで通り audio/mpeg', async () => {
    const { audio } = await parseMultipartResponse(turnResponse('audio/mpeg', 'ID3mp3'))
    expect(audio.type).toBe('audio/mpeg')
    expect(audio.size).toBe('ID3mp3'.length)
  })
})
