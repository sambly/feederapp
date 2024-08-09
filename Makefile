# Makefile


# Установка приватной библиотеки локально  и запуск go mod tidy 

ifeq (,$(wildcard .env))
  $(error .env file not found)
endif
include .env
export $(shell sed 's/=.*//' .env)


.PHONY: all setup deps
all: setup deps


# Имя проекта
PROJECT_NAME := feeder-app

# Закрытый репозиторий
PRIVATE_REPO := github.com/sambly/exchangeService

# Переменные окружения
export GOPRIVATE := github.com/sambly
export GIT_TERMINAL_PROMPT := 1

# Команды
setup:
	@echo "Setting up environment..."
	@if [ -z "$$GITHUB_TOKEN" ]; then echo "Error: GITHUB_TOKEN is not set"; exit 1; fi
	@git config --global url."https://$$GITHUB_TOKEN@github.com/".insteadOf "https://github.com/"

deps:
	@echo "Fetching dependencies..."
	@go get $(PRIVATE_REPO)
	@go mod tidy



