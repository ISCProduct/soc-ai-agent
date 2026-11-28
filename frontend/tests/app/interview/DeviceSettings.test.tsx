/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import DeviceSettings from '@/app/interview/components/lobby/DeviceSettings'

const device = (kind: MediaDeviceKind, deviceId: string, label: string) =>
  ({ kind, deviceId, label, groupId: '' }) as MediaDeviceInfo

const DEVICES = [
  device('audioinput', 'mic-a', '内蔵マイク'),
  device('audioinput', 'mic-b', 'USBマイク'),
  device('audiooutput', 'spk-a', '内蔵スピーカー'),
  device('audiooutput', 'spk-b', 'イヤホン'),
  device('videoinput', 'cam-a', '内蔵カメラ'),
  device('videoinput', 'cam-b', 'USBカメラ'),
]

const stream = { getAudioTracks: () => [] } as unknown as MediaStream

function renderSettings(overrides: Partial<React.ComponentProps<typeof DeviceSettings>> = {}) {
  const props = {
    stream,
    micId: '',
    cameraId: '',
    speakerId: '',
    onSelectMic: jest.fn(),
    onSelectCamera: jest.fn(),
    onSelectSpeaker: jest.fn(),
    ...overrides,
  }
  render(<DeviceSettings {...props} />)
  return props
}

let enumerateDevices: jest.Mock
beforeEach(() => {
  enumerateDevices = jest.fn().mockResolvedValue(DEVICES)
  Object.defineProperty(navigator, 'mediaDevices', {
    value: { enumerateDevices, addEventListener: jest.fn(), removeEventListener: jest.fn() },
    configurable: true,
  })
})
afterEach(() => {
  delete (globalThis as { AudioContext?: unknown }).AudioContext
})

describe('DeviceSettings', () => {
  it('つながっている機器を並べ、選ぶと選んだ機器の ID を渡す', async () => {
    const props = renderSettings()

    await waitFor(() => expect(screen.getByRole('option', { name: 'USBマイク' })).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('マイク'), { target: { value: 'mic-b' } })
    fireEvent.change(screen.getByLabelText('カメラ'), { target: { value: 'cam-b' } })

    expect(props.onSelectMic).toHaveBeenCalledWith('mic-b')
    expect(props.onSelectCamera).toHaveBeenCalledWith('cam-b')
  })

  it('許可前（ストリームが無い）は機器を読みに行かない', () => {
    renderSettings({ stream: null })

    expect(enumerateDevices).not.toHaveBeenCalled()
  })

  it('スピーカーを選べないブラウザ（AudioContext に setSinkId が無い）では、スピーカー欄を出さない', async () => {
    ;(globalThis as { AudioContext?: unknown }).AudioContext = class {}
    renderSettings()

    await waitFor(() => expect(enumerateDevices).toHaveBeenCalled())
    expect(screen.queryByLabelText('スピーカー')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'スピーカーをテスト' })).toBeInTheDocument()
  })

  it('スピーカーを選べるブラウザでは、スピーカー欄から選べる', async () => {
    class FakeAudioContext {
      setSinkId() { return Promise.resolve() }
    }
    ;(globalThis as { AudioContext?: unknown }).AudioContext = FakeAudioContext
    const props = renderSettings()

    await waitFor(() => expect(screen.getByRole('option', { name: 'イヤホン' })).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('スピーカー'), { target: { value: 'spk-b' } })

    expect(props.onSelectSpeaker).toHaveBeenCalledWith('spk-b')
  })

  it('選んでいた機器が抜かれたら既定に戻す', async () => {
    const props = renderSettings({ micId: 'mic-gone', cameraId: 'cam-a' })

    await waitFor(() => expect(props.onSelectMic).toHaveBeenCalledWith(''))
    expect(props.onSelectCamera).not.toHaveBeenCalled()
  })
})
