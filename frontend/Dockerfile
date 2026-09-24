FROM node:22-alpine

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

RUN if [ "$NODE_ENV" = "production" ]; then npm run build; fi

EXPOSE 3000

CMD ["sh", "-c", "if [ \"$NODE_ENV\" = \"development\" ]; then npm run dev -- --hostname 0.0.0.0; else npm start; fi"]
