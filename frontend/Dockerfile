FROM node:20-alpine

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

RUN if [ "$NODE_ENV" = "production" ]; then npm run build; fi

EXPOSE 3000

CMD ["sh", "-c", "if [ \"$NODE_ENV\" = \"development\" ]; then npm run dev -- --hostname 0.0.0.0; else npm start; fi"]
