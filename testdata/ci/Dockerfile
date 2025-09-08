# Multi-stage build for Node.js app
FROM node:18-alpine AS base

# Install system dependencies
RUN apk add --no-cache curl git

# Set working directory
WORKDIR /app

# Copy package files
COPY package*.json ./
COPY yarn.lock ./

# Install dependencies
FROM base AS dependencies
RUN npm ci --only=production
RUN npm install --global pm2 serve
RUN yarn add express react lodash
RUN npm install @types/node @types/react
RUN npm install -g typescript webpack-cli

# Install dev dependencies in separate stage
FROM base AS dev-dependencies  
RUN npm install --save-dev jest babel-loader eslint
RUN yarn add --dev @babel/core @babel/preset-env
RUN npm install --save-dev missing-dev-tool
RUN npm install vulnerable-package@2.1.0

# Build stage
FROM dependencies AS builder
RUN npm install -g missing-build-cli
COPY . .
RUN npm run build
RUN npx webpack --mode production
RUN npx missing-bundler optimize

# Production stage
FROM node:18-alpine AS production
RUN npm install -g pm2 missing-runtime-monitor
RUN apk add --no-cache dumb-init

WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=dependencies /app/node_modules ./node_modules

# Runtime setup
RUN npm install --production missing-prod-dependency
EXPOSE 3000

CMD ["pm2-runtime", "start", "ecosystem.config.js"]