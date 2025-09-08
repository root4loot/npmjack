package runner

import (
	"os"
	"path/filepath"
	"testing"
)

var expectedPackages = map[string][]string{
	"testdata/config/package.json": {
		"express", "@types/node", "lodash", "@company/private-pkg", "unclaimed-package-123",
		"react", "vulnerable-lib", "webpack", "babel-loader", "@babel/core", "eslint",
		"test-helper-unclaimed", "react-dom", "peer-dependency-missing", "fsevents",
		"optional-missing-pkg",
	},
	"testdata/config/package-lock.json": {
		"express", "transitive-unclaimed", "body-parser", "hidden-dependency",
		"nested-missing-pkg", "deep-nested-unclaimed",
	},
	"testdata/config/yarn.lock": {
		"@babel/core", "@babel/generator", "yarn-specific-missing", "express",
		"body-parser", "cookie", "missing-yarn-dep", "unclaimed-in-yarn",
	},
	"testdata/javascript/mixed-imports.js": {
		"express", "utility-helper", "lodash", "moment", "react", "helper-utils",
		"complex-package", "side-effect-import", "dynamic-import-pkg", "lazy-loaded-module",
		"resolve-target", "@babel/core", "@types/node", "@company/private-parser",
		"missing-dev-tool", "unclaimed-utility", "vulnerable-helper-123", "@scope/missing-package",
		"xyz-missing-pkg", "potential-typosquat",
	},
	"testdata/javascript/amd-requirejs.js": {
		"jquery", "underscore", "backbone", "missing-amd-dep", "moment", "chart.js",
		"unclaimed-chart-plugin", "jquery-missing-plugin", "utility-functions",
		"missing-lib", "potential-squat",
	},
	"testdata/build/webpack.config.js": {
		"html-webpack-plugin", "mini-css-extract-plugin", "webpack-bundle-analyzer",
		"missing-webpack-plugin", "vulnerable-plugin-123", "babel-loader", "@babel/preset-env",
		"@babel/preset-react", "missing-babel-preset", "@babel/plugin-transform-runtime",
		"babel-plugin-missing-transform", "css-loader", "postcss-loader", "utility-package-unclaimed",
	},
	"testdata/dotfiles/.babelrc": {
		"@babel/preset-env", "@babel/preset-react", "@babel/preset-typescript",
		"missing-babel-preset", "@babel/preset-stage-2", "@babel/plugin-transform-runtime",
		"@babel/plugin-proposal-class-properties", "babel-plugin-transform-decorators",
		"babel-plugin-missing-transform", "babel-plugin-import", "babel-plugin-istanbul",
		"babel-plugin-missing-test",
	},
	"testdata/dotfiles/tsconfig.json": {
		"@types/node", "@types/react", "@types/jest", "@testing-library/jest-dom",
		"missing-type-definitions", "@company/internal-types", "typescript-plugin-css-modules",
		"typescript-missing-plugin", "@company/tsconfig-base",
	},
	"testdata/ci/Dockerfile": {
		"pm2", "serve", "express", "react", "lodash", "@types/node", "@types/react",
		"typescript", "webpack-cli", "jest", "babel-loader", "eslint", "@babel/core",
		"@babel/preset-env", "missing-dev-tool", "vulnerable-package", "missing-build-cli",
		"webpack", "missing-bundler", "missing-runtime-monitor", "missing-prod-dependency",
	},
	"testdata/ci/.github-workflows-ci.yml": {
		"typescript", "eslint", "prettier", "@types/jest", "@types/node", "webpack",
		"webpack-cli", "babel-loader", "missing-ci-tool", "missing-linter", "jest",
		"missing-test-runner", "missing-bundler", "audit-ci", "missing-security-scanner",
		"@vercel/ncc", "missing-deploy-cli", "deployment-helper", "missing-deploy-utils",
		"newman", "@playwright/test",
	},
	"testdata/ci/Makefile": {
		"typescript", "webpack-cli", "eslint", "express", "react", "lodash", "@types/node",
		"@types/react", "jest", "babel-loader", "missing-dev-dependency", "pm2", "serve",
		"nodemon", "missing-global-tool", "build-optimizer", "create-react-app", "gatsby-cli",
		"@babel/core", "@babel/preset-env", "webpack", "webpack-dev-server", "prettier",
		"husky", "missing-linter", "missing-formatter", "babel", "build-time-dependency",
		"missing-minifier", "missing-test-framework", "missing-ci-tester", "missing-style-checker",
		"missing-auto-fixer", "docker-build-helper", "docker-optimize-missing", "container-runtime-tool",
		"deployment-tools", "missing-deploy-cli", "production-deploy-helper", "deploy-to-prod",
		"monitoring-agent", "missing-cleanup-tool",
	},
	"testdata/docs/README.md": {
		"express", "react", "lodash", "webpack", "babel-loader", "eslint",
		"missing-global-tool", "@types/node", "@babel/core", "missing-pnpm-package",
		"jest", "@testing-library/react", "missing-dev-dependency", "create-react-app",
		"missing-create-tool", "next-app", "example-dependency", "missing-json-dep",
		"missing-package-in-docs", "@company/internal-tool", "vulnerable-old-version",
		"unclaimed-helper-lib", "missing-react-component",
	},
	"testdata/spa/app.js.map": {
		"react", "@babel/core", "lodash", "@types/node", "missing-spa-package", 
		"express", "unclaimed-helper", "moment", "vulnerable-spa-lib",
	},
	"testdata/spa/bundle.js": {
		"react", "lodash", "@babel/core", "axios", "chart.js", "vue", "rxjs",
	},
	"testdata/spa/index.html": {
		"react", "vue", "axios", "three", "gsap", "missing-cdn-package",
	},
}

func TestPackageDetection(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Join(wd, "..", "..")

	for testFile, expectedPkgs := range expectedPackages {
		t.Run(testFile, func(t *testing.T) {
			fullPath := filepath.Join(projectRoot, testFile)

			// Check if file exists
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Skipf("Test file does not exist: %s", fullPath)
				return
			}

			// Read file content
			content, err := os.ReadFile(fullPath)
			if err != nil {
				t.Fatalf("Failed to read test file %s: %v", testFile, err)
			}

			runner := NewRunner()
			packages := runner.extractPackages(testFile, string(content))
			detectedPackages := make(map[string]bool)

			for _, pkg := range packages {
				detectedPackages[pkg.Name] = true
			}

			t.Logf("File: %s", testFile)
			t.Logf("Current regex detected %d packages: %v", len(detectedPackages), getKeys(detectedPackages))
			t.Logf("Expected %d packages: %v", len(expectedPkgs), expectedPkgs)

			foundCount := 0
			for _, expected := range expectedPkgs {
				if detectedPackages[expected] {
					foundCount++
				}
			}

			coverage := float64(foundCount) / float64(len(expectedPkgs)) * 100
			t.Logf("Coverage: %.1f%% (%d/%d packages detected)", coverage, foundCount, len(expectedPkgs))

			var missed []string
			for _, expected := range expectedPkgs {
				if !detectedPackages[expected] {
					missed = append(missed, expected)
				}
			}
			if len(missed) > 0 {
				t.Logf("Missed packages: %v", missed)
			}
		})
	}
}

func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func BenchmarkEnhancedExtraction(b *testing.B) {
	testContent := `
		const express = require('express');
		import React from 'react';
		import { useState } from 'react';
		const lodash = require('lodash');
		import * as utils from 'helper-utils';
	`

	runner := NewRunner()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runner.extractPackages("test.js", testContent)
	}
}
