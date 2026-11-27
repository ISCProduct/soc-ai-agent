# Whisper.cpp HTTP server（POST /inference）。
# 公式イメージ ghcr.io/ggml-org/whisper.cpp:main を土台に、モデル取得と起動を entrypoint に寄せる。
# Backend/internal/aifailover の LocalSTT が叩く契約に合わせる。
#
# 最終ステージは非root（automation/test/dockerfile-nonroot-test.sh / #1477）。

FROM ghcr.io/ggml-org/whisper.cpp:main

COPY entrypoint.sh /entrypoint.sh

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
