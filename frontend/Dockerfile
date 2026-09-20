FROM node:22-alpine

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

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
