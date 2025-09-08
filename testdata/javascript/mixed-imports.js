// CommonJS requires
const express = require('express');
const fs = require('fs');
const { someFunc } = require('utility-helper');
const path = require('path');
var lodash = require('lodash');
let moment = require('moment');

// ES6 imports
import React from 'react';
import { useState, useEffect } from 'react';
import * as utils from 'helper-utils';
import defaultExport, { namedExport } from 'complex-package';
import 'side-effect-import';

// Dynamic imports
const module = await import('dynamic-import-pkg');
import('lazy-loaded-module').then(mod => console.log(mod));

// Require with resolve
const resolved = require.resolve('resolve-target');

// Scoped packages
import '@babel/core';
const typescript = require('@types/node');
import { Parser } from '@company/private-parser';

// Potential unclaimed packages
require('missing-dev-tool');
import 'unclaimed-utility';
const helper = require('vulnerable-helper-123');
import('@scope/missing-package');

// Minified/obfuscated patterns (common in bundled code)
require("a"),require("b"),require("xyz-missing-pkg")
import('x');import('y');import('potential-typosquat');