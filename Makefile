.DEFAULT_GOAL := help

GOLANGCI_VERSION := 2.13.2
GOLANGCI_DIR := $(CURDIR)/build/tools/golangci-lint-$(GOLANGCI_VERSION)
GOLANGCI_LINT := $(GOLANGCI_DIR)/golangci-lint
AIR_VERSION := 1.67.4
AIR := $(CURDIR)/build/tools/air-$(AIR_VERSION)/air

.PHONY: help setup dev dev-server up start down stop restart status logs ready test test-all test-web test-server test-cli test-desktop lint-desktop build-desktop-ui build-desktop test-process test-dev lint format build build-web build-server build-cli integration check skills-check docker-up docker-down docker-logs

help: ## 显示可用命令（默认）
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  make %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: $(GOLANGCI_LINT) $(AIR) ## 按锁文件安装开发依赖
	pnpm install --frozen-lockfile
	cargo fetch --manifest-path cli/Cargo.toml --locked
	cargo fetch --manifest-path desktop/Cargo.toml --locked
	cd server && go mod download

dev: ## 前台启动 Go + Vite，Ctrl+C 关闭
	pnpm dev

dev-server: $(AIR) ## 前台启动 Go，源码变更自动编译重启
	$(AIR) -c .air.toml

$(AIR):
	mkdir -p "$(dir $(AIR))"
	GOBIN="$(dir $(AIR))" go install github.com/air-verse/air@v$(AIR_VERSION)

up: $(AIR) ## 后台启动 Go + Vite，等待就绪（重复执行不重启）
	@node --env-file-if-exists=.env scripts/dev.mjs up

start: up ## up 的别名

down: ## 停止本项目后台开发进程，保留日志
	@node --env-file-if-exists=.env scripts/dev.mjs down

stop: down ## down 的别名

restart: ## 先关闭再后台启动（重新构建 Go）
	$(MAKE) down
	$(MAKE) up

status: ## 查看后台开发进程状态与地址
	@node --env-file-if-exists=.env scripts/dev.mjs status

logs: ## 跟随后台开发日志，Ctrl+C 仅退出查看
	@node --env-file-if-exists=.env scripts/dev.mjs logs

ready: ## 完整检查通过后，后台启动全部开发服务
	$(MAKE) check
	$(MAKE) up

test: test-server test-cli test-web ## 三端测试（Go / Rust / React）

test-all: check ## 全部检查、测试、构建和集成验证

test-web:
	pnpm --dir web test

test-server:
	cd server && go test -race -count=1 ./...

test-cli:
	cargo test --manifest-path cli/Cargo.toml --locked

test-process: build-server $(AIR) ## 验证前台进程退出与清理
	node --test tests/process.test.mjs

test-dev: $(AIR) ## 验证后台启动、停止、重载、重复启动和失败清理
	node --test tests/dev.test.mjs tests/reload.test.mjs

$(GOLANGCI_LINT):
	mkdir -p "$(GOLANGCI_DIR)"
	curl --fail --silent --show-error --location "https://raw.githubusercontent.com/golangci/golangci-lint/v$(GOLANGCI_VERSION)/install.sh" -o "$(GOLANGCI_DIR)/install.sh"
	sh "$(GOLANGCI_DIR)/install.sh" -b "$(GOLANGCI_DIR)" "v$(GOLANGCI_VERSION)"

lint: $(GOLANGCI_LINT) ## 三端静态检查与格式检查
	pnpm exec biome check --error-on-warnings .
	pnpm --dir web typecheck
	cd server && "$(GOLANGCI_LINT)" fmt --diff ./...
	cd server && "$(GOLANGCI_LINT)" run ./...
	cd server && go mod tidy -diff
	cargo fmt --manifest-path cli/Cargo.toml --all -- --check
	cargo clippy --manifest-path cli/Cargo.toml --locked --all-targets -- -D warnings

format: $(GOLANGCI_LINT) ## 自动格式化三端代码
	pnpm exec biome check --write .
	cd server && "$(GOLANGCI_LINT)" fmt ./...
	cargo fmt --manifest-path cli/Cargo.toml --all

build: build-web build-server build-cli build-demo ## 构建 Web、Go、Rust release 和本地演示工具

build-web:
	pnpm --dir web build

build-server:
	mkdir -p build
	cd server && CGO_ENABLED=0 go build -trimpath -o ../build/pluginpocket-server ./cmd/pluginpocket-server

build-cli:
	cargo build --manifest-path cli/Cargo.toml --locked --release

.PHONY: build-demo
build-demo: ## 构建本地演示数据与 mock MCP 工具
	mkdir -p build
	cd server && go build -trimpath -o ../build/pluginpocket-demo ./cmd/pluginpocket-demo

integration: build ## 构建并验证真实 HTTP / CLI 集成
	node --test tests/integration.test.mjs

skills-check:
	test -f .agents/skills/superpowers/test-driven-development/SKILL.md
	test -f .agents/skills/ponytail/SKILL.md
	test -f .agents/skills/impeccable/SKILL.md
	test -x .agents/skills/impeccable/scripts/impeccable

check: ## 完整 harness，任一步失败即停止
	node --test tests/release-preflight.test.mjs
	$(MAKE) database-check
	$(MAKE) redis-check
	$(MAKE) skills-check
	$(MAKE) lint
	$(MAKE) lint-desktop
	$(MAKE) test
	$(MAKE) test-desktop
	$(MAKE) test-e2e
	$(MAKE) test-process
	$(MAKE) test-dev
	node --test tests/integration.test.mjs

docker-up: ## 构建并后台启动 Compose 容器
	docker compose up --build -d

docker-down: ## 关闭 Compose 容器（保留数据卷）
	docker compose down

docker-logs: ## 跟随 Compose 日志
	docker compose logs --tail=100 -f

# Native desktop checks require GTK/WebKit development libraries on Linux.
build-desktop-ui:
	pnpm --dir desktop/ui build

lint-desktop: build-desktop-ui ## 桌面界面类型检查、Rust 格式与静态检查
	cargo fmt --manifest-path desktop/Cargo.toml --all -- --check
	cargo clippy --manifest-path desktop/Cargo.toml --locked --all-targets -- -D warnings

test-desktop: build-desktop-ui ## 桌面组件、受限本地命令与真实原生 bridge 测试
	pnpm --dir desktop/ui test
	cargo test --manifest-path desktop/Cargo.toml --locked --no-default-features
	cargo test --manifest-path desktop/Cargo.toml --locked

build-desktop: build-desktop-ui ## 构建 Linux 桌面 release 与 Debian 安装包
	pnpm --dir desktop/ui tauri build --bundles deb -- --locked

.PHONY: database-check redis-check test-product test-e2e benchmark db-up redis-up

database-check: ## 要求真实 PostgreSQL；避免业务测试被跳过仍报告成功
	@test -n "$$PLUGINPOCKET_TEST_DATABASE_URL" || { echo "Set PLUGINPOCKET_TEST_DATABASE_URL to an isolated PostgreSQL 18 test database."; exit 1; }

test-product: database-check build-server build-cli build-demo ## 真实数据库、Go 服务、Rust bridge 全链路
	cd server && PLUGINPOCKET_SERVER_BINARY="$(CURDIR)/build/pluginpocket-server" PLUGINPOCKET_CLI_BINARY="$(CURDIR)/cli/target/release/pluginpocket" go test -race ./cmd/pluginpocket-server -run TestProductJourney -count=1 -v

test-e2e: database-check redis-check build build-desktop ## 真实 Web（生产/开发）、CLI、桌面 bridge、数据库与故障恢复
	cd server && PLUGINPOCKET_WEB_E2E=1 PLUGINPOCKET_SERVER_BINARY="$(CURDIR)/build/pluginpocket-server" PLUGINPOCKET_CLI_BINARY="$(CURDIR)/cli/target/release/pluginpocket" PLUGINPOCKET_DESKTOP_BINARY="$(CURDIR)/desktop/target/release/pluginpocket-desktop" go test -race ./cmd/pluginpocket-server -run TestProductJourney -count=1 -v

benchmark: database-check ## PostgreSQL 同钱包串行/并发扣费基准（不含网络上游延迟）
	cd server && go test ./internal/store -run '^$$' -bench BenchmarkReserveFinish -benchmem -benchtime=2s -count=3

load-test: database-check build-server ## 真实子进程端到端压测（QPS/分位数/计量一致性），需 PLUGINPOCKET_SERVER_BINARY
	cd server && PLUGINPOCKET_SERVER_BINARY="$(CURDIR)/build/pluginpocket-server" go test -tags load -count=1 -run TestLoadGatewayQPS -timeout 8m -v ./cmd/pluginpocket-server

db-up: ## 仅启动本地 PostgreSQL，供源码开发
	docker compose up -d db

redis-check: ## 要求真实 Redis，避免共享缓存验证被静默跳过
	@test -n "$$PLUGINPOCKET_TEST_REDIS_URL" || { echo "Set PLUGINPOCKET_TEST_REDIS_URL to an isolated Redis test instance."; exit 1; }

redis-up: ## 仅启动本地 Redis，供源码开发
	docker compose up -d redis
