# --- Configuration ---
# Le nom du module Go, tel que défini dans votre go.mod
MODULE_NAME := $(shell go list -m)

# Dossier de destination pour tous les binaires
DIST_DIR := dist

# Découvre automatiquement tous les plugins dans le dossier plugins/
PLUGINS := $(shell find plugins -mindepth 1 -maxdepth 1 -type d -exec basename {} \;)

# Ajoute le dossier des binaires Go au PATH utilisé par make pour trouver les outils
GOPATH := $(shell go env GOPATH)
export PATH := $(GOPATH)/bin:$(PATH)

# --- Cibles Principales ---
.PHONY: all build clean rebuild permissions

# La cible par défaut qui reconstruit tout proprement
all: rebuild

# Construit tous les plugins
build:
	@$(foreach plugin,$(PLUGINS), \
		make build-plugin PLUGIN=$(plugin); \
	)

clean:
	@echo "--> Cleaning up project..."
	@rm -rf $(DIST_DIR)
	@echo "Cleanup complete."

# Reconstruit tout le projet depuis le début
rebuild: clean all

# --- Cibles de Build Spécifiques (utilisées par le CI/CD) ---
.PHONY: build-plugin

# Construit un plugin spécifique, identifié par la variable PLUGIN
# Exemple d'utilisation : make build-plugin PLUGIN=core
build-plugin:
	@echo "--> Building plugin: $(PLUGIN)..."
	@mkdir -p $(DIST_DIR)/$(PLUGIN)
	go build -ldflags="-s -w" -o $(DIST_DIR)/$(PLUGIN)/orkestra-plugin-$(PLUGIN) ./plugins/$(PLUGIN)

# Rend tous les binaires de plugins compilés exécutables
permissions:
	@chmod +x $(DIST_DIR)/*/orkestra-plugin-*

