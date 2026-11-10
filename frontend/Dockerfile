# 24.22.1 は 2026-10-21 のセキュリティリリース。浮動タグの 24-alpine は 24.22.0 のまま残る。
# 23 系は 2025-06-01 に EOL で、この修正は配信されない。
FROM node:24.22.1-alpine

WORKDIR /app

# 非root実行(#1477)。node イメージ同梱の node ユーザー(uid 1000)をそのまま使う。
# npm install / npm run build も node で回すので、後段の巨大な chown -R が要らない。
#
# .next と node_modules は compose 側で匿名ボリュームにされる。
# Docker は「イメージ内に存在するパス」の所有権をボリュームへ引き継ぐため、
# .next を空でも先に作っておかないと root 所有のボリュームができ、
# next dev / next start がビルド成果物を書けずに起動直後で落ちる。
RUN mkdir -p /app/.next && chown -R node:node /app

USER node

COPY --chown=node:node package*.json ./
RUN npm install

COPY --chown=node:node . .

# ARG/ENVでNODE_ENV=productionをnpm installより前にスコープさせると、devDependencies
# (typescript等)がインストールされずwebpackのパスエイリアス解決が壊れる。そのため
# ARG宣言自体をnpm installより後に置き、ビルド直前まで存在させない。
ARG NODE_ENV=development
ENV NODE_ENV=$NODE_ENV

# NEXT_PUBLIC_*はクライアントバンドルにビルド時埋め込みされるため、実行時のdocker-compose
# .envでは反映されない。環境ごとに --build-arg で渡してビルドし直す必要がある。
ARG NEXT_PUBLIC_BACKEND_URL
ENV NEXT_PUBLIC_BACKEND_URL=$NEXT_PUBLIC_BACKEND_URL
# NEXT_PUBLIC_* はビルド時にバンドルへ埋め込まれる。ここで渡さないと
# ブラウザ側の Sentry は DSN を読めず、SDK を積むだけで一度も初期化されない。
ARG NEXT_PUBLIC_SENTRY_DSN
ENV NEXT_PUBLIC_SENTRY_DSN=$NEXT_PUBLIC_SENTRY_DSN
ARG NEXT_PUBLIC_APP_ENV
ENV NEXT_PUBLIC_APP_ENV=$NEXT_PUBLIC_APP_ENV
ARG NEXT_PUBLIC_SENTRY_RELEASE
ENV NEXT_PUBLIC_SENTRY_RELEASE=$NEXT_PUBLIC_SENTRY_RELEASE
# Live2D面接官を有効にするか。既定(未指定)は Avatar2D のまま。
# Cubism Core とモデルはライセンス上リポジトリに置けず、デプロイ時に S3 から
# ビルドコンテキストへ展開する（`docs/wiki/live2d-assets.md` 参照）。資産が無いまま
# true にすると毎セッション404を取ってから Avatar2D へ落ちるので、資産を置く
# ワークフローと必ずセットで渡す。
ARG NEXT_PUBLIC_LIVE2D_ENABLED
ENV NEXT_PUBLIC_LIVE2D_ENABLED=$NEXT_PUBLIC_LIVE2D_ENABLED

RUN if [ "$NODE_ENV" = "production" ]; then npm run build; fi

EXPOSE 3000

CMD ["sh", "-c", "if [ \"$NODE_ENV\" = \"development\" ]; then npm run dev -- --hostname 0.0.0.0; else npm start; fi"]
