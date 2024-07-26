# Makefile

# Параметры репозитория
REPO_URL = https://github.com/sambly/exchangeService.git
REPO_DIR = external/exchangeService
BRANCH = develop
COMMIT = 00be8d07565d3fc3bcded184c9ee0c051c27e9fc
PKG_DIR = pkg

# Цель по умолчанию
all: deps

# Цель для клонирования репозитория и извлечения папки pkg
clone-repo:
	@if [ ! -d "$(REPO_DIR)" ]; then \
		echo "Cloning repository..."; \
		git clone --branch $(BRANCH) $(REPO_URL) $(REPO_DIR); \
	fi
	cd $(REPO_DIR) && git fetch --all && git checkout $(COMMIT);

# Цель для извлечения только папки pkg с использованием sparse-checkout
sparse-checkout:
	@if [ -d "$(REPO_DIR)" ]; then \
		cd $(REPO_DIR); \
		git config core.sparseCheckout true; \
		echo "$(PKG_DIR)" > .git/info/sparse-checkout; \
		git read-tree -mu HEAD; \
	else \
		echo "Directory $(REPO_DIR) does not exist."; \
	fi

# Цель для установки зависимостей
deps: clone-repo sparse-checkout
	@echo "Dependencies prepared."

# Цель для чистки
clean:
	@echo "Cleaning up..."
	go clean
	rm -rf $(REPO_DIR)
