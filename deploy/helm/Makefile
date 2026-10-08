# =============================================================================
# e-commerce_shop — деплой в Kubernetes
# =============================================================================
# Все команды выполняются из корня репозитория.
#   make -C deploy/helm <target>
# или
#   cd deploy/helm && make <target>
#
# Переопределяемые переменные:
#   HELM=helm                       бинарь helm
#   NAMESPACE=ecommerce             namespace
#   RELEASE_INFRA=ecommerce-infra   имя релиза инфраструктуры
#   RELEASE_APP=ecommerce           имя релиза приложения
#   RELEASE_OBS=observability       имя релиза observability
#   REGISTRY=registry.example.com   хост реестра образов
#   PROJECT=krokozabra213           проект/путь в реестре (владелец GHCR)
#   TAG=v1.0.0                     тег образов (по умолчанию — appVersion чарта)
#   ENV=dev|prod                    профиль values
# =============================================================================

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

HELM        ?= helm
KUBECTL     ?= kubectl
NAMESPACE   ?= ecommerce
ENV         ?= dev
REGISTRY    ?= registry.example.com
PROJECT     ?= krokozabra213
# Тег по умолчанию = appVersion чарта приложения (единый источник истины,
# см. deploy/helm/ecommerce-shop/Chart.yaml). Переопределяется: make TAG=1.2.3 ...
TAG         ?= $(shell sed -nE 's/^appVersion:[[:space:]]*"?([^"[:space:]]+)"?[[:space:]]*$$/\1/p' $(CHART_APP)/Chart.yaml | head -1)

RELEASE_INFRA ?= ecommerce-infra
RELEASE_APP   ?= ecommerce
RELEASE_OBS   ?= observability

ROOT        := $(shell cd ../.. && pwd)
# Пути относительно каталога этого Makefile (deploy/helm), т.е. при запуске
# `make -C deploy/helm ...`. Раньше здесь стоял префикс `../`, из-за чего
# цели искали несуществующие deploy/infra, deploy/ecommerce-shop и т.д.
CHART_INFRA := infra
CHART_APP   := ecommerce-shop
CHART_OBS   := observability

VALUES_INFRA := $(CHART_INFRA)/values-$(ENV).yaml
VALUES_APP   := $(CHART_APP)/values-$(ENV).yaml
VALUES_OBS   := $(CHART_OBS)/values-$(ENV).yaml

# Приватный ключ JWT для dev. В prod ключ берётся из существующего Secret'а.
JWT_KEY     ?= $(ROOT)/services/auth-service/certs/jwt-private.pem
# Для lint/template подставляется ФИКТИВНЫЙ ключ: настоящий PEM не нужен, а его
# попадание в build/ (пусть и git-ignored) нежелательно.
JWT_SET_FILE = $(if $(wildcard $(JWT_KEY)),--set-file secrets.values.jwtPrivateKey=$(JWT_KEY),)
JWT_DUMMY    = --set secrets.values.jwtPrivateKey=VALIDATION-ONLY-NOT-A-REAL-KEY

BUILD_DIR   := build

.PHONY: help
help: ## Показать список целей
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_.-]+:.*?## / {printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# -----------------------------------------------------------------------------
# Проверки (без кластера)
# -----------------------------------------------------------------------------

.PHONY: lint
lint: ## helm lint всех чартов
	$(HELM) lint $(CHART_INFRA)
	$(HELM) lint $(CHART_APP) $(JWT_DUMMY)
	$(HELM) lint $(CHART_OBS)

.PHONY: template
template: ## Отрендерить все чарты во все профили в build/
	@mkdir -p $(BUILD_DIR)
	$(HELM) template $(RELEASE_INFRA) $(CHART_INFRA) -f $(VALUES_INFRA) > $(BUILD_DIR)/infra-$(ENV).yaml
	$(HELM) template $(RELEASE_APP) $(CHART_APP) -f $(VALUES_APP) $(JWT_DUMMY) > $(BUILD_DIR)/app-$(ENV).yaml
	$(HELM) template $(RELEASE_OBS) $(CHART_OBS) -f $(VALUES_OBS) > $(BUILD_DIR)/observability-$(ENV).yaml
	@echo "==> build/:"
	@ls -la $(BUILD_DIR)

.PHONY: validate
validate: lint template ## lint + render + простая проверка YAML (dev и prod)
	./scripts/validate-config.sh
	@echo "OK"

.PHONY: validate-prod
validate-prod: ## То же, но рендер/линт на values-prod.yaml
	$(MAKE) ENV=prod lint template

.PHONY: kubeconform
kubeconform: template ## Валидация схем (если установлен kubeconform)
	@command -v kubeconform >/dev/null || { echo "kubeconform не установлен"; exit 1; }
	kubeconform -strict -summary -ignore-missing-schemas $(BUILD_DIR)/*.yaml

# -----------------------------------------------------------------------------
# Образы
# -----------------------------------------------------------------------------

.PHONY: build-images
build-images: ## Собрать образы сервисов и миграций
	REGISTRY=$(REGISTRY) PROJECT=$(PROJECT) TAG=$(TAG) ./scripts/build-images.sh

.PHONY: push-images
push-images: ## Собрать и запушить образы
	REGISTRY=$(REGISTRY) PROJECT=$(PROJECT) TAG=$(TAG) PUSH=1 ./scripts/build-images.sh

.PHONY: kind-load
kind-load: ## Загрузить образы в kind (без реестра)
	./scripts/build-images.sh
	kind load docker-image $(shell ./scripts/build-images.sh --print-tags)

.PHONY: minikube-load
minikube-load: ## Загрузить образы в minikube (без реестра)
	./scripts/build-images.sh
	minikube image load $(shell ./scripts/build-images.sh --print-tags)

# -----------------------------------------------------------------------------
# Установка
# -----------------------------------------------------------------------------

.PHONY: namespace
namespace: ## Создать namespace
	$(KUBECTL) create namespace $(NAMESPACE) --dry-run=client -o yaml | $(KUBECTL) apply -f -

.PHONY: install-infra
install-infra: namespace ## Установить инфраструктуру и дождаться готовности
	$(HELM) upgrade --install $(RELEASE_INFRA) $(CHART_INFRA) \
		--namespace $(NAMESPACE) -f $(VALUES_INFRA) \
		--wait --timeout 15m

.PHONY: install-observability
install-observability: namespace ## Установить observability-стек
	$(HELM) upgrade --install $(RELEASE_OBS) $(CHART_OBS) \
		--namespace $(NAMESPACE) -f $(VALUES_OBS) \
		--wait --timeout 10m

.PHONY: install-app
install-app: namespace ## Установить приложение (миграции выполняются хуками)
	$(HELM) upgrade --install $(RELEASE_APP) $(CHART_APP) \
		--namespace $(NAMESPACE) -f $(VALUES_APP) $(JWT_SET_FILE) \
		--wait --timeout 15m

.PHONY: install
install: install-infra install-observability install-app ## Установить всё по порядку
	@echo
	@echo "Готово. Состояние:"
	@$(KUBECTL) get pods -n $(NAMESPACE)

.PHONY: uninstall
uninstall: ## Удалить релизы (PVC остаются — см. README)
	-$(HELM) uninstall $(RELEASE_APP) -n $(NAMESPACE)
	-$(HELM) uninstall $(RELEASE_OBS) -n $(NAMESPACE)
	-$(HELM) uninstall $(RELEASE_INFRA) -n $(NAMESPACE)

.PHONY: status
status: ## Статус подов/сервисов/ингрессов
	$(KUBECTL) get pods,svc,ingress,hpa,pdb -n $(NAMESPACE)

.PHONY: logs
logs: ## Логи сервиса (SVC=api-gateway)
	$(KUBECTL) logs -n $(NAMESPACE) -l app.kubernetes.io/component=$(SVC) --tail=200 -f

.PHONY: migrations
migrations: ## Логи Job'ов миграций
	$(KUBECTL) get jobs -n $(NAMESPACE)
	@for j in $$($(KUBECTL) get jobs -n $(NAMESPACE) -o name); do \
		echo "=== $$j"; $(KUBECTL) logs -n $(NAMESPACE) "$$j" --tail=50; \
	done

.PHONY: clean
clean: ## Удалить build/
	rm -rf $(BUILD_DIR)
