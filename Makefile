# Makefile

# Установка приватной библиотеки локально и запуск go mod tidy 
ifeq (,$(wildcard .env))
  $(error .env file not found)
endif
include .env
export $(shell sed 's/=.*//' .env)

# Имя проекта
PROJECT_NAME := feeder-app

# Закрытый репозиторий
PRIVATE_REPO := github.com/sambly/exchangeService

# Переменные окружения
export GOPRIVATE := github.com/sambly
export GIT_TERMINAL_PROMPT := 1


# Настройка окружения Go 
.PHONY: setup-env prepare install-deps install-deps-develop
setup-env: prepare install-deps

prepare:
	@echo "Setting up environment..."
	@if [ -z "$$GITHUB_TOKEN" ]; then echo "Error: GITHUB_TOKEN is not set"; exit 1; fi
	@git config --global url."https://$$GITHUB_TOKEN@github.com/".insteadOf "https://github.com/"
install-deps:
	@echo "Fetching dependencies..."
	@go get $(PRIVATE_REPO)
	@go mod tidy
# использовать ветку develop
install-deps-develop:
	@echo "Fetching dependencies..."
	@go get $(PRIVATE_REPO)@develop
	@go mod tidy


# Линтеры 
.PHONY: lint lint-golangci install-linter-golangci lint-env install-linter-env lint-all-env fix-env fix-all-env compare-envs compare-all-envs
lint: lint-go-fmt lint-golangci lint-env


lint-go-fmt:
	gofmt -s -w .

lint-golangci:
	golangci-lint run || true

install-linter-golangci:
	@echo "Установка golangci..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest


# Основные задачи lint-env 
lint-env: lint-all-env fix-all-env compare-all-envs

# ./bin  нужно добавить путь если не добавлен 
install-linter-env:
	@echo "Установка dotenv-linter..."
	curl -sSfL https://raw.githubusercontent.com/dotenv-linter/dotenv-linter/master/install.sh | sh -s -- -b usr/local/bin v3.3.0

# Проверка основного .env файла
lint-env:
	@echo "Linting .env file..."
	dotenv-linter lint .env --skip UnorderedKey || true

# Проверка всех .env файлов в проекте
lint-all-env:
	@echo "Linting all .env files..."
	dotenv-linter lint $(shell find . -type f -name ".env*") --skip UnorderedKey || true

# Исправление ошибок в основном .env файле
fix-env:
	@echo "Fixing .env file..."
	dotenv-linter fix .env --skip UnorderedKey || true

# Исправление ошибок во всех .env файлах
fix-all-env:
	@echo "Fixing all .env files..."
	dotenv-linter fix $(shell find . -type f -name ".env*") --skip UnorderedKey || true


# Сравнение .env и .env.example в каждой папке
compare-envs:
	@echo "Comparing .env and .env.example files..."
	dotenv-linter compare .env .env.example || true

# Сравнение всех .env и .env.example файлов в проекте
compare-all-envs:
	@echo "Comparing all .env and .env.example files..."
	@for dir in $$(find . -type d); do \
		if [ -f $$dir/.env ] && [ -f $$dir/.env.example ]; then \
			dotenv-linter compare $$dir/.env $$dir/.env.example || true; \
		fi \
	done

# Создание и отправка Git-тега
.PHONY: tag
tag:
	@if [ -z "$(version)" ]; then echo "Error: version is not set. Usage: make tag version=v1.1.1"; exit 1; fi
	@echo "Creating and pushing tag $(version)..."
	git tag -a $(version) -m "$(version)"
	git push origin $(version)