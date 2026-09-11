FROM node:26.8.2-alpine AS web
WORKDIR /src
RUN npm install -g pnpm@12.3.4
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml .npmrc ./
COPY web/package.json web/package.json
COPY desktop/ui/package.json desktop/ui/package.json
RUN pnpm install --frozen-lockfile
COPY web web
RUN pnpm --dir web build

FROM golang:1.27.1-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /pluginpocket-server ./cmd/pluginpocket-server

FROM scratch
WORKDIR /app
COPY --from=server /pluginpocket-server /app/pluginpocket-server
COPY --from=server /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=web /src/web/dist /app/web/dist
USER 65532:65532
ENV PLUGINPOCKET_ADDR=0.0.0.0:8787
EXPOSE 8787
ENTRYPOINT ["/app/pluginpocket-server"]
