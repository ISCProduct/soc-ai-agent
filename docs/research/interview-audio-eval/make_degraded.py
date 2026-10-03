#!/usr/bin/env python3
"""劣化音声フィクスチャの生成器（#1603 ①実測）。

既存の合成音声8ケース（manifest.jsonl）から、録音環境が悪いときを模した
劣化版を再現可能に作る。生成物は追跡しない（100ファイル規模になるため）。
コミットするのはこのスクリプトと manifest_degraded.jsonl だけ。

    python3 make_degraded.py                      # 生成 + 実測レベル表を出す
    python3 make_degraded.py --manifest-only      # マニフェストだけ作り直す
    python3 make_degraded.py --only noisy         # 条件名の前方一致で絞る

強度は「狙った値」ではなく必ず実測して合わせる。特に SNR は amix の重みだけでは
決まらないので、(1)音声のRMSを測る (2)目標SNRになる雑音ゲインを計算する
(3)生成した雑音をもう一度測って狙い通りか確認する、の3段で扱う。
雑音は seed 固定なので、同じ入力からは毎回同じ音声ができる。

実発話・PIIは扱わない。入力は合成音声のみ（source は常に synthetic）。
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

HERE = Path(__file__).resolve().parent
SRC_MANIFEST = HERE / "manifest.jsonl"
OUT_MANIFEST = HERE / "manifest_degraded.jsonl"
OUT_DIR = HERE / "audio" / "degraded"
TMP_DIR = HERE / "audio" / ".degraded-tmp"

# WAVは既存フィクスチャと同じ 16kHz mono。
WAV_ARGS = ["-c:a", "pcm_s16le", "-ar", "16000", "-ac", "1"]
# WEBMは本番がブラウザから送る形式に寄せる。
# frontend の useInterviewSession は audioBitsPerSecond: 128000 を要求しているため
# 既存フィクスチャ（実測 29kb/s）ではなく 128k を既定にする。
WEBM_BITRATE = "128k"
WEBM_ARGS = ["-c:a", "libopus", "-ar", "48000", "-ac", "1"]


@dataclass(frozen=True)
class Variant:
    """劣化条件1種。condition がそのまま sttbench の集計キーになる。"""

    condition: str
    family: str
    # 音声側に掛けるフィルタ（雑音混合の前段）。
    pre: str = "anull"
    # 雑音混合の後段に掛けるフィルタ。combo で使う。
    post: str = "anull"
    # 目標SNR(dB)。None なら雑音を混ぜない。
    snr_db: float | None = None
    noise_color: str = "white"
    # opus 往復で潰す場合のビットレート。None なら往復しない。
    opus_bitrate: str | None = None
    note: str = ""


def variants() -> list[Variant]:
    """劣化条件の一覧。想定する実環境ごとに強度を振る。"""
    out: list[Variant] = [
        Variant("clean", "clean", note="劣化なしの対照（同じ経路で作り直したもの）"),
    ]
    # 自宅・カフェ・家族の生活音。白色とピンクで分ける。
    for color in ("white", "pink"):
        for snr in (20, 10, 5, 0):
            out.append(
                Variant(
                    f"noisy-{color}-snr{snr}",
                    "noisy",
                    snr_db=float(snr),
                    noise_color=color,
                    note=f"{color}雑音 SNR {snr}dB",
                )
            )
    # 硬い壁の部屋・風呂場のような反響。aecho は本物の残響ではなく離散エコー。
    out += [
        Variant("reverb-weak", "reverb", pre="aecho=0.8:0.7:20:0.25", note="弱い反響"),
        Variant("reverb-mid", "reverb", pre="aecho=0.8:0.85:40|70:0.35|0.25", note="中程度の反響"),
        Variant("reverb-strong", "reverb", pre="aecho=0.8:0.9:50|90|140:0.5|0.4|0.3", note="強い反響"),
    ]
    # マイクが遠い・小声。
    for db in (12, 24, 36):
        out.append(Variant(f"quiet-{db}db", "quiet", pre=f"volume=-{db}dB", note=f"-{db}dB 減衰"))
    # マイク入力が過大で歪む。s16 に落とす時点で頭が潰れる。
    out += [
        Variant("clipped-light", "clipped", pre="volume=10dB", note="+10dB で軽度に潰す"),
        Variant("clipped-heavy", "clipped", pre="volume=24dB", note="+24dB で重度に潰す"),
    ]
    # 安価なヘッドセット・電話品質。
    out += [
        Variant("lowband-phone", "lowband", pre="highpass=f=300,lowpass=f=3400", note="300-3400Hz"),
        Variant("lowband-narrow", "lowband", pre="highpass=f=500,lowpass=f=2500", note="500-2500Hz"),
    ]
    # 回線が細くコーデックが潰れる。本番は WEBM(opus) なのでここが最も本番に近い。
    for br in ("24k", "12k", "6k"):
        out.append(Variant(f"lowbitrate-{br}", "lowbitrate", opus_bitrate=br, note=f"opus {br}"))
    # 通信の瞬断・パケットロス。120ms の無音を周期的に差し込む。
    for period in ("3", "1.5", "0.8"):
        key = period.replace(".", "")
        out.append(
            Variant(
                f"dropout-{key}s",
                "dropout",
                pre=rf"volume=0:eval=frame:enable='lt(mod(t\,{period})\,0.12)'",
                note=f"{period}秒ごとに120msの無音",
            )
        )
    # 早口。
    for tempo in ("1.3", "1.6"):
        out.append(Variant(f"fast-{tempo}x", "fast", pre=f"atempo={tempo}", note=f"{tempo}倍速"))
    # 実際の悪い環境は単独では来ないので組み合わせも入れる。
    out.append(
        Variant(
            "combo-noisy10-lowband",
            "combo",
            snr_db=10.0,
            noise_color="pink",
            post="highpass=f=300,lowpass=f=3400",
            note="ピンク雑音 SNR 10dB + 300-3400Hz",
        )
    )
    return out


def run(args: list[str]) -> str:
    """ffmpeg/ffprobe を実行し、標準出力+標準エラーを返す。"""
    p = subprocess.run(args, capture_output=True, text=True)
    if p.returncode != 0:
        sys.stderr.write(" ".join(args) + "\n" + p.stderr[-2000:] + "\n")
        raise SystemExit(f"コマンド失敗: {args[0]}")
    return p.stdout + p.stderr


MEAN_RE = re.compile(r"mean_volume:\s*(-?[\d.]+) dB")
MAX_RE = re.compile(r"max_volume:\s*(-?[\d.]+) dB")
HIST0_RE = re.compile(r"histogram_0db:\s*(\d+)")


def measure(path: Path) -> tuple[float, float, int]:
    """mean_volume(dBFS), max_volume(dBFS), 0dB に貼り付いたサンプル数を返す。

    `-f null -` では取れないことがあるため出力は /dev/null に捨てる。
    """
    out = run(["ffmpeg", "-hide_banner", "-i", str(path), "-af", "volumedetect", "-f", "null", "/dev/null"])
    mean = MEAN_RE.search(out)
    mx = MAX_RE.search(out)
    if not mean or not mx:
        raise SystemExit(f"音量を測れませんでした: {path}")
    h0 = HIST0_RE.search(out)
    return float(mean.group(1)), float(mx.group(1)), int(h0.group(1)) if h0 else 0


def duration(path: Path) -> float:
    out = run(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(path)])
    return float(out.strip().splitlines()[0])


def seed_of(case_id: str, condition: str) -> int:
    """雑音の seed。ケースと条件から決めるので生成は再現可能。"""
    h = 0
    for ch in f"{case_id}/{condition}":
        h = (h * 131 + ord(ch)) % 2147483647
    return h


def noise_graph(v: Variant, dur: float, seed: int, gain_db: float | None) -> str:
    g = f"anoisesrc=c={v.noise_color}:r=16000:d={dur:.3f}:a=1.0:seed={seed}"
    if gain_db is not None:
        g += f",volume={gain_db:.2f}dB"
    return g


def render(src: Path, v: Variant, dst: Path, webm: Path, seed: int) -> dict:
    """1ケース1条件ぶんの wav/webm を作り、実測値を返す。"""
    TMP_DIR.mkdir(parents=True, exist_ok=True)
    dst.parent.mkdir(parents=True, exist_ok=True)

    src_mean, src_max, _ = measure(src)
    stage = dst  # opus 往復がある場合だけ中間ファイルを挟む
    if v.opus_bitrate:
        stage = TMP_DIR / (dst.stem + ".pre.wav")

    achieved_snr: float | None = None
    noise_mean: float | None = None
    if v.snr_db is None:
        run(["ffmpeg", "-y", "-hide_banner", "-i", str(src), "-af", f"{v.pre},{v.post}", *WAV_ARGS, str(stage)])
    else:
        # 早口などで長さが変わる条件と雑音は併用しないので、元の長さで雑音を作る。
        dur = duration(src)
        # (1) 無加工の雑音のRMSを測る → (2) 目標SNRとの差からゲインを決める
        probe = TMP_DIR / (dst.stem + ".noise0.wav")
        run(["ffmpeg", "-y", "-hide_banner", "-f", "lavfi", "-i", noise_graph(v, dur, seed, None), *WAV_ARGS, str(probe)])
        n0, _, _ = measure(probe)
        target = src_mean - v.snr_db
        gain = target - n0
        # (3) ゲインを掛けた雑音を測り直して、狙い通りに落ちているか確認する
        noise = TMP_DIR / (dst.stem + ".noise.wav")
        run(["ffmpeg", "-y", "-hide_banner", "-f", "lavfi", "-i", noise_graph(v, dur, seed, gain), *WAV_ARGS, str(noise)])
        noise_mean, _, _ = measure(noise)
        achieved_snr = src_mean - noise_mean
        run(
            [
                "ffmpeg", "-y", "-hide_banner",
                "-i", str(src), "-i", str(noise),
                "-filter_complex",
                f"[0:a]{v.pre}[s];[s][1:a]amix=inputs=2:duration=first:weights='1 1':normalize=0,{v.post}[m]",
                "-map", "[m]", *WAV_ARGS, str(stage),
            ]
        )
        probe.unlink(missing_ok=True)
        noise.unlink(missing_ok=True)

    if v.opus_bitrate:
        # 低ビットレートは opus で潰してから wav へ戻す。WAV側も同じ劣化を受ける。
        ogg = TMP_DIR / (dst.stem + ".opus.webm")
        run(["ffmpeg", "-y", "-hide_banner", "-i", str(stage), *WEBM_ARGS, "-b:a", v.opus_bitrate, str(ogg)])
        run(["ffmpeg", "-y", "-hide_banner", "-i", str(ogg), *WAV_ARGS, str(dst)])
        ogg.unlink(missing_ok=True)
        stage.unlink(missing_ok=True)

    br = v.opus_bitrate or WEBM_BITRATE
    run(["ffmpeg", "-y", "-hide_banner", "-i", str(dst), *WEBM_ARGS, "-b:a", br, str(webm)])

    mean, mx, clipped = measure(dst)
    return {
        "condition": v.condition,
        "src_mean_db": src_mean,
        "src_max_db": src_max,
        "mean_db": mean,
        "max_db": mx,
        "clipped_samples": clipped,
        "noise_mean_db": noise_mean,
        "target_snr_db": v.snr_db,
        "achieved_snr_db": achieved_snr,
        "duration_sec": duration(dst),
        "wav_bytes": dst.stat().st_size,
        "webm_bytes": webm.stat().st_size,
    }


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--only", default="", help="condition の前方一致で絞る（例: noisy / reverb）")
    ap.add_argument("--manifest-only", action="store_true", help="音声を作らずマニフェストだけ書き出す")
    ap.add_argument("--levels", default=str(TMP_DIR.parent / "degraded_levels.tsv"), help="実測レベルの出力先")
    args = ap.parse_args()

    base = [json.loads(line) for line in SRC_MANIFEST.read_text(encoding="utf-8").splitlines() if line.strip()]
    vs = [v for v in variants() if v.condition.startswith(args.only)]
    if not vs:
        raise SystemExit(f"条件が一致しません: {args.only}")

    lines: list[str] = []
    levels: list[dict] = []
    for v in vs:
        for c in base:
            cid = f"{c['id']}__{v.condition}"
            wav = OUT_DIR / f"{cid}.wav"
            webm = OUT_DIR / f"{cid}.webm"
            if not args.manifest_only:
                m = render(HERE / c["file"], v, wav, webm, seed_of(c["id"], v.condition))
                m["id"] = cid
                levels.append(m)
                print(
                    f"{cid:58s} mean {m['mean_db']:7.1f}dB max {m['max_db']:6.1f}dB "
                    f"SNR {('%.1f' % m['achieved_snr_db']) if m['achieved_snr_db'] is not None else '-':>6s} "
                    f"{m['duration_sec']:6.2f}s"
                )
            lines.append(
                json.dumps(
                    {
                        "id": cid,
                        "file": str(wav.relative_to(HERE)),
                        "webm_file": str(webm.relative_to(HERE)),
                        # 劣化しても話した内容は変わらない。ここが評価の軸。
                        "reference_text": c["reference_text"],
                        "tags": [*c.get("tags", []), "degraded", v.family],
                        "checks": c.get("checks", []),
                        "condition": v.condition,
                        "source": "synthetic",
                    },
                    ensure_ascii=False,
                )
            )

    OUT_MANIFEST.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"\nマニフェスト: {OUT_MANIFEST} ({len(lines)}件)")

    if levels:
        cols = [
            "id", "condition", "src_mean_db", "mean_db", "max_db", "clipped_samples",
            "noise_mean_db", "target_snr_db", "achieved_snr_db", "duration_sec", "wav_bytes", "webm_bytes",
        ]
        rows = ["\t".join(cols)]
        rows += ["\t".join("" if r.get(k) is None else str(r.get(k)) for k in cols) for r in levels]
        Path(args.levels).write_text("\n".join(rows) + "\n", encoding="utf-8")
        print(f"実測レベル: {args.levels}")
    if TMP_DIR.exists() and not any(TMP_DIR.iterdir()):
        TMP_DIR.rmdir()


if __name__ == "__main__":
    main()
