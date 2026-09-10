.PHONY: setup dev dev-server test test-web test-server test-cli test-process lint format build build-web build-server build-cli integration check skills-check

setup:
	pnpm install --frozen-lockfile
	cargo fetch --manifest-path cli/Cargo.toml --locked

dev:
	pnpm dev

dev-server: build-server
	./build/loadout-server

test: test-server test-cli test-web

test-web:
	pnpm --dir web test

test-server:
	cd server && go test -race -count=1 ./...

test-cli:
	cargo test --manifest-path cli/Cargo.toml --locked

test-process: build-server
	node --test tests/process.test.mjs

lint:
	pnpm exec biome check --error-on-warnings .
	pnpm --dir web typecheck
	cd server && test -z "$$(gofmt -l .)"
	cd server && go vet ./...
	cargo fmt --manifest-path cli/Cargo.toml --all -- --check
	cargo clippy --manifest-path cli/Cargo.toml --locked --all-targets -- -D warnings

format:
	pnpm exec biome check --write .
	cd server && gofmt -w .
	cargo fmt --manifest-path cli/Cargo.toml --all

build: build-web build-server build-cli

build-web:
	pnpm --dir web build

build-server:
	mkdir -p build
	cd server && CGO_ENABLED=0 go build -trimpath -o ../build/loadout-server ./cmd/loadout-server

build-cli:
	cargo build --manifest-path cli/Cargo.toml --locked --release

integration: build
	node --test tests/integration.test.mjs

skills-check:
	test -f .agents/skills/superpowers/test-driven-development/SKILL.md
	test -f .agents/skills/ponytail/SKILL.md
	test -f .agents/skills/impeccable/SKILL.md
	test -x .agents/skills/impeccable/scripts/impeccable

check:
	$(MAKE) skills-check
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) build
	$(MAKE) test-process
	node --test tests/integration.test.mjs
