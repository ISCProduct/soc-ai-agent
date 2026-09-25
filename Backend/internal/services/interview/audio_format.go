package interview

import "bytes"

// AudioFormat は録音データのコンテナ種別。
//
// OpenAI の Transcription API は送信ファイル名の拡張子で形式を判断する。
// 以前は録音が必ず WebM である前提で "audio.webm" を固定送信していたが、
// ブラウザによって MediaRecorder が出せる形式は違う（Safari は WebM を
// 作れず MP4 になる）。形式と拡張子が食い違うと、API 側が復号に失敗するか
// 誤った解釈をして認識精度が落ちる。
//
// クライアントの申告する Content-Type は使わない。ブラウザの実装差や
// プロキシで書き換わることがあり、実体と一致する保証が無いため、
// 受け取ったバイト列そのものから判定する。
type AudioFormat struct {
	// Ext は API へ送るファイル名の拡張子（先頭のドットを含まない）。
	Ext string
	// MIME は観測ログ用。認識そのものには使わない。
	MIME string
}

// 判定できなかった場合の既定。
//
// 実際の録音の大半は WebM なので、未知の形式は WebM として送る。
// 誤っていても API がエラーを返すだけで、面接は既存のフォールバック
// （聞き取れなかった扱い）で継続する。
var audioFormatUnknown = AudioFormat{Ext: "webm", MIME: "audio/webm"}

// DetectAudioFormat は録音バイト列からコンテナ種別を判定する。
//
// 対応する形式は MediaRecorder が実際に出しうるものに絞る。
// 網羅性より「よくある形式を取り違えないこと」を優先する。
func DetectAudioFormat(data []byte) AudioFormat {
	switch {
	// EBML ヘッダ。WebM / Matroska に共通。
	case len(data) >= 4 && bytes.Equal(data[:4], []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return AudioFormat{Ext: "webm", MIME: "audio/webm"}

	// ISO BMFF。先頭4バイトはボックスサイズなので 4〜8 バイト目を見る。
	// Safari の MediaRecorder はここに入る。
	case len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")):
		return AudioFormat{Ext: "mp4", MIME: "audio/mp4"}

	// Ogg（Opus / Vorbis）。Firefox が出すことがある。
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("OggS")):
		return AudioFormat{Ext: "ogg", MIME: "audio/ogg"}

	// RIFF/WAVE。調査用フィクスチャや外部ツール経由で来る。
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WAVE")):
		return AudioFormat{Ext: "wav", MIME: "audio/wav"}

	// MPEG audio。ID3 タグ付きと素の同期ワードの両方。
	case len(data) >= 3 && bytes.Equal(data[:3], []byte("ID3")):
		return AudioFormat{Ext: "mp3", MIME: "audio/mpeg"}
	case len(data) >= 2 && data[0] == 0xFF && data[1]&0xE0 == 0xE0:
		return AudioFormat{Ext: "mp3", MIME: "audio/mpeg"}

	// FLAC。
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("fLaC")):
		return AudioFormat{Ext: "flac", MIME: "audio/flac"}
	}
	return audioFormatUnknown
}

// AudioFilename は API へ送るファイル名を返す。
func (f AudioFormat) AudioFilename() string { return "audio." + f.Ext }
