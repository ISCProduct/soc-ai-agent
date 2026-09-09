#!/bin/bash
# コンテナ(CPUのみ)でのSTT速度を測る。ホスト(GPU)との比較用。
set -uo pipefail
WAV="${WAV:-/work/voice/ans.wav}"
REF="${REF:-/work/voice/ans.txt}"
THREADS="${THREADS:-$(nproc)}"
# ffmpeg を入れていないので音声長は環境変数で受け取る
DUR="${DUR:?音声長(秒)を DUR で渡してください}"

echo "CPU: $(nproc) コア / スレッド指定: $THREADS / 音声長: ${DUR}秒"
printf "%-20s %8s %10s %8s\n" "モデル" "秒" "実時間比" "一致率"
for MODEL in "$@"; do
  BIN="/work/models/ggml-$MODEL.bin"
  [ -f "$BIN" ] || { printf "%-20s %8s\n" "$MODEL" "(未取得)"; continue; }
  S=$(python3 -c 'import time;print(time.time())')
  whisper-cli -m "$BIN" -f "$WAV" -l ja -np -nt -t "$THREADS" 2>/dev/null > "/work/voice/cpu_$MODEL.txt"
  E=$(python3 -c 'import time;print(time.time())')
  python3 - "$S" "$E" "$DUR" "$REF" "/work/voice/cpu_$MODEL.txt" "$MODEL" <<'PY'
import sys, re, difflib
s,e,d = float(sys.argv[1]), float(sys.argv[2]), float(sys.argv[3])
n = lambda p: re.sub(r'[\s、。,.]', '', open(p, encoding="utf-8").read())
try: acc = f"{difflib.SequenceMatcher(None, n(sys.argv[4]), n(sys.argv[5])).ratio()*100:.1f}%"
except Exception: acc = "-"
print(f"{sys.argv[6]:<20} {e-s:8.2f} {d/(e-s):9.1f}x {acc:>8}")
PY
done
