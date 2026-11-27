# Whisper.cpp HTTP server（POST /inference）。
# 公式イメージ ghcr.io/ggml-org/whisper.cpp:main を土台に、モデル取得と起動を entrypoint に寄せる。
# Backend/internal/aifailover の LocalSTT が叩く契約に合わせる。

FROM ghcr.io/ggml-org/whisper.cpp:main

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV WHISPER_MODEL=tiny
EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
