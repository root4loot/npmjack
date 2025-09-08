// TypeScript specific imports
import type { ComponentType } from 'react';
import type { Config } from 'missing-config-types';

/// <reference types="node" />
/// <reference types="missing-types-package" />

// Standard imports with types
import express, { Request, Response } from 'express';
import * as fs from 'fs';
import { readFile } from 'fs/promises';

// Import with type annotations
import { createServer }: { createServer: Function } from 'http';
import defaultExport: DefaultType from 'typed-missing-package';

// Re-exports (common in index files)
export { default as Component } from 'component-library';
export * from 'utility-functions';
export type { TypeDef } from 'missing-type-defs';

// Module declarations (often found in .d.ts files)
declare module 'untyped-package' {
  export function someFunction(): void;
}

declare module 'missing-module-declaration' {
  export interface SomeInterface {
    prop: string;
  }
}

// Import with type-only imports
import { type TypeOnly, regularImport } from 'mixed-import-package';
import type { OnlyType } from 'type-only-import';

// Namespace imports
import * as Namespace from 'namespace-package';
import Namespace2 = require('legacy-namespace-import');

// Dynamic imports with await
const module = await import('dynamic-typescript-module');
const { utils } = await import('missing-dynamic-utils');