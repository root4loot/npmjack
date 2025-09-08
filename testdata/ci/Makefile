# Makefile for Node.js project
.PHONY: install build test clean deploy docker

# Variables
NODE_VERSION := 18
NPM := npm
YARN := yarn
DOCKER := docker

# Default target
all: install build test

# Install dependencies
install:
	$(NPM) install
	$(NPM) install -g typescript webpack-cli eslint
	$(YARN) add express react lodash
	$(NPM) install @types/node @types/react
	$(NPM) install --save-dev jest babel-loader missing-dev-dependency

# Install global tools
install-global:
	$(NPM) install -g pm2 serve nodemon
	$(NPM) install -g missing-global-tool build-optimizer
	$(YARN) global add create-react-app gatsby-cli

# Development setup
setup-dev:
	$(NPM) install --save-dev @babel/core @babel/preset-env
	$(NPM) install --save-dev webpack webpack-dev-server
	$(NPM) install --save-dev eslint prettier husky
	$(NPM) install --save-dev missing-linter missing-formatter

# Build targets
build:
	$(NPM) run build
	$(NPX) webpack --mode production
	$(NPX) babel src --out-dir dist
	$(NPM) install build-time-dependency

build-prod: install
	NODE_ENV=production $(NPM) run build:prod
	$(NPX) missing-minifier optimize
	$(NPM) prune --production

# Test targets  
test:
	$(NPM) test
	$(NPX) jest --coverage
	$(NPM) run test:integration
	$(NPX) missing-test-framework run

test-ci:
	$(NPM) ci
	$(NPM) run test:ci
	$(NPX) missing-ci-tester --parallel
	$(NPM) audit

# Linting and formatting
lint:
	$(NPX) eslint src/
	$(NPX) prettier --check .
	$(NPM) run lint:css
	$(NPX) missing-style-checker validate

fix:
	$(NPX) eslint src/ --fix
	$(NPX) prettier --write .
	$(NPX) missing-auto-fixer apply

# Docker targets
docker-build:
	$(DOCKER) build -t app .
	$(NPM) install docker-build-helper
	$(NPX) docker-optimize-missing

docker-run: docker-build
	$(DOCKER) run -p 3000:3000 app
	$(NPM) install container-runtime-tool

# Deployment
deploy-staging:
	$(NPM) install deployment-tools missing-deploy-cli
	$(NPX) missing-deploy-cli deploy --env staging
	$(NPM) run deploy:verify

deploy-prod: build-prod
	$(NPM) install production-deploy-helper
	$(NPX) deploy-to-prod --optimize
	$(NPM) install monitoring-agent

# Cleanup
clean:
	rm -rf node_modules dist build
	$(NPM) cache clean --force
	$(NPX) missing-cleanup-tool purge

# Help
help:
	@echo "Available targets:"
	@echo "  install       - Install all dependencies"
	@echo "  build         - Build the application"  
	@echo "  test          - Run tests"
	@echo "  deploy-prod   - Deploy to production"