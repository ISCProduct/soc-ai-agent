package interview

import "testing"

// 実ファイルの先頭バイト（docs/research/interview-audio-eval/audio/ より採取）。
// 音声そのものは git 管理外なので、ヘッダだけをここに持つ。
var (
	// clean-introduction.webm: 1a45dfa39f4286810142f781
	realWebMHeader = []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01, 0x42, 0xF7, 0x81}
	// clean-introduction.wav: 5249464646eb050057415645
	realWAVHeader = []byte{'R', 'I', 'F', 'F', 0x46, 0xEB, 0x05, 0x00, 'W', 'A', 'V', 'E'}
)

func TestDetectAudioFormat(t *testing.T) {
	// Safari の MediaRecorder が出す MP4。先頭4バイトはボックスサイズ。
	mp4 := append([]byte{0x00, 0x00, 0x00, 0x20}, []byte("ftypisom0000")...)

	tests := []struct {
		name    string
		data    []byte
		wantExt string
	}{
		{"実ファイルのWebMヘッダ", realWebMHeader, "webm"},
		{"実ファイルのWAVヘッダ", realWAVHeader, "wav"},
		{"Safari の MP4", mp4, "mp4"},
		{"Firefox の Ogg", append([]byte("OggS"), make([]byte, 24)...), "ogg"},
		{"ID3付きMP3", append([]byte("ID3"), make([]byte, 16)...), "mp3"},
		{"素のMP3同期ワード", []byte{0xFF, 0xFB, 0x90, 0x00}, "mp3"},
		{"FLAC", append([]byte("fLaC"), make([]byte, 16)...), "flac"},

		// 判定できないものは WebM として送る。API がエラーを返すだけで、
		// 面接は既存のフォールバックで継続する。
		{"未知の形式はwebm扱い", []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x00, 0x11}, "webm"},
		{"空データ", nil, "webm"},
		{"短すぎるデータ", []byte{0x1A}, "webm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectAudioFormat(tt.data)
			if got.Ext != tt.wantExt {
				t.Errorf("Ext = %q, want %q", got.Ext, tt.wantExt)
			}
			if got.MIME == "" {
				t.Error("MIME が空")
			}
			if want := "audio." + tt.wantExt; got.AudioFilename() != want {
				t.Errorf("AudioFilename = %q, want %q", got.AudioFilename(), want)
			}
		})
	}
}

// RIFF だが WAVE でないもの（AVI 等）を WAV と誤判定しない。
func TestDetectAudioFormat_RIFFButNotWave(t *testing.T) {
	avi := []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'A', 'V', 'I', ' '}
	if got := DetectAudioFormat(avi); got.Ext == "wav" {
		t.Errorf("RIFF/AVI を wav と判定している")
	}
}

// WebM のマジックに1バイトでも差があれば WebM と断定しない。
// ここが緩いと、壊れたデータを WebM として送り続けることになる。
func TestDetectAudioFormat_NearMissIsNotWebM(t *testing.T) {
	broken := []byte{0x1A, 0x45, 0xDF, 0xA4, 0x9F, 0x42}
	got := DetectAudioFormat(broken)
	// 既定は webm だが、それは「判定できなかった」結果であって
	// EBML と認識したからではない。既定値そのものであることを確認する。
	if got != audioFormatUnknown {
		t.Errorf("マジックが違うのに判定している: %+v", got)
	}
}
