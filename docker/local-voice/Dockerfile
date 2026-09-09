# ローカル音声スタック(STT)の検証用イメージ。
#
# 目的は「GPUが無い環境でどこまで出るか」を測ること。
# 本番(ECS Fargate)はGPUを持たず、macOS の Docker も Metal にアクセスできないため、
# ここで出る数字はホスト(Apple Silicon GPU)ではなく本番に近い側の実測値になる。
#
# 公式のビルド済みイメージ(ghcr.io/ggml-org/whisper.cpp)は arm64 が無く、
# エミュレーションでは計測が歪むためソースからビルドする。
# ffmpeg は入れない。依存が重くメモリ5.3GBのVMでビルドが落ちるため、
# 音声長は呼び出し側から渡す。入力は16kHzモノラルWAVに変換済みのものを使う。
FROM debian:bookworm-slim AS build

RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential cmake git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG WHISPER_REF=v1.7.4
RUN git clone --depth 1 --branch ${WHISPER_REF} https://github.com/ggerganov/whisper.cpp /src \
    && cmake -S /src -B /src/build -DCMAKE_BUILD_TYPE=Release -DWHISPER_BUILD_TESTS=OFF \
    && cmake --build /src/build --config Release -j 2

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends python3 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /src/build/bin/whisper-cli /usr/local/bin/whisper-cli
COPY --from=build /src/build/src/libwhisper.so* /usr/local/lib/
COPY --from=build /src/build/ggml/src/libggml*.so /usr/local/lib/
RUN ldconfig

WORKDIR /work
COPY bench.sh /usr/local/bin/bench
RUN chmod +x /usr/local/bin/bench

CMD ["sleep", "infinity"]
