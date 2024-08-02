# Makefile


# Загружаем переменные из .env файла
ifeq (,$(wildcard ./.env))
  $(error .env file not found)
endif

include .env
export $(shell sed 's/=.*//' .env)


# Параметры репозитория
REPO_URL = https://github.com/sambly/exchangeService.git
REPO_DIR = external/exchangeService
PKG_DIR = pkg
FILES = go.mod go.sum

# Цель по умолчанию
all: clean deps

# Цель для клонирования репозитория и извлечения папки pkg
clone-repo:
	@if [ ! -d "$(REPO_DIR)" ]; then \
		echo "Cloning repository..."; \
		git clone --branch $(BRANCH) $(REPO_URL) $(REPO_DIR); \
	fi
	cd $(REPO_DIR) && git fetch --all && git checkout $(COMMIT);

# Цель для извлечения только папки pkg и файлов go.mod, go.sum с использованием sparse-checkout
sparse-checkout:
	@if [ -d "$(REPO_DIR)" ]; then \
		cd $(REPO_DIR); \
		git config core.sparseCheckout true; \
		echo "$(PKG_DIR)" > .git/info/sparse-checkout; \
		echo "go.mod" >> .git/info/sparse-checkout; \
		echo "go.sum" >> .git/info/sparse-checkout; \
		git read-tree -mu HEAD; \
 	else \
   		echo "Directory $(REPO_DIR) does not exist."; \
 	fi

# Цель для установки зависимостей
deps: clone-repo sparse-checkout
	@echo "Dependencies prepared."
	 cd $(REPO_DIR) && go mod tidy


# Цель для чистки
clean:
	@echo "Cleaning up..."
	go clean
	rm -rf $(REPO_DIR)
