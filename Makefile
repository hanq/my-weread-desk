SHELL := bash
APP   := my-weread-desk
BIN   := build/bin/$(APP)$(if $(filter windows,$(shell go env GOOS)),.exe,)

.PHONY: help dev build build-clean test lint fmt tidy run clean frontend-install frontend-build frontend-dev

help: ## 显示可用目标
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

dev: ## 开发模式（热重载）
	wails dev

build: ## 生产构建
	wails build

build-clean: ## 生产构建（清理缓存）
	wails build -clean

run: build ## 构建后直接运行
	./$(BIN)

test: ## 运行 Go 单元测试
	go test ./...

test-race: ## 运行 Go 单元测试（含竞态检测）
	go test -race ./...

lint: ## 静态检查（Go 格式、vet、前端类型）
	@out=$$(gofmt -l . | grep -v '^frontend/' || true); \
	if [ -n "$$out" ]; then echo "gofmt 需要修复："; echo "$$out"; exit 1; fi
	go vet ./...
	cd frontend && npx tsc --noEmit

fmt: ## 格式化 Go 代码
	gofmt -w .

tidy: ## 整理 Go 依赖
	go mod tidy

frontend-install: ## 安装前端依赖
	cd frontend && npm install --no-audit --no-fund

frontend-build: ## 只构建前端
	cd frontend && npm run build

frontend-dev: ## 只启动前端（使用内置假数据后端，不访问网络）
	cd frontend && npm run dev

clean: ## 清理构建产物
	rm -rf build/bin frontend/dist
