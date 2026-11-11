package interview

import "testing"

// OpenAI の TTS は mp3、OpenAI 障害時のローカル（VOICEVOX）は WAV（#1833）。
func TestAudioContentType(t *testing.T) {
	if got := audioContentType([]byte("RIFF\x24\x00\x00\x00WAVEfmt ")); got != "audio/wav" {
		t.Fatalf("WAV = %q", got)
	}
	if got := audioContentType([]byte("ID3\x03mp3")); got != "audio/mpeg" {
		t.Fatalf("mp3 = %q", got)
	}
	if got := audioContentType(nil); got != "audio/mpeg" {
		t.Fatalf("nil = %q", got)
	}
}
