# Whisper.cpp HTTP server（POST /inference）。
# 公式イメージを土台に、モデル取得と起動を entrypoint に寄せる。
# Backend/internal/aifailover の LocalSTT が叩く契約に合わせる。
#
# 最終ステージは非root（automation/test/dockerfile-nonroot-test.sh / #1477）。
# digest は 2026-11-27 時点の ghcr.io/ggml-org/whisper.cpp:main（linux/amd64）。

FROM ghcr.io/ggml-org/whisper.cpp:main@sha256:a02c5bcd922e6d3c3372b23ab84f87a64756efb7cd547372f8033e91fdfbe822

COPY entrypoint.sh /entrypoint.sh

# 公式イメージの PATH は /app/build/bin 中心。スクリプトは /app/models にある。
ENV PATH="/app/models:/app/build/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

# /models は volume マウント先。非root で download-ggml-model.sh が書けるよう先に用意する。
RUN groupadd --gid 10001 app \
 && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/app --shell /bin/sh app \
 && mkdir -p /models \
 && chmod +x /entrypoint.sh \
 && chown -R app:app /models /entrypoint.sh /home/app

USER app

ENV WHISPER_MODEL=tiny
EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
