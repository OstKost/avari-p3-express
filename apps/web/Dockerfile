FROM node:20-alpine AS builder
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm run build
ARG APP_VERSION=development
ARG APP_COMMIT=unknown
RUN node -e 'require("fs").writeFileSync("dist/version.json", JSON.stringify({version:process.argv[1],commit:process.argv[2]}))' "$APP_VERSION" "$APP_COMMIT"
FROM nginx:1.28-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
