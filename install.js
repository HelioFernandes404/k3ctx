#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const os = require('os');

const src = path.join(__dirname, '.claude', 'skills', 'k3ctx');
const dest = path.join(os.homedir(), '.claude', 'skills', 'k3ctx');

if (!fs.existsSync(src)) {
  console.error(`Error: skill source not found at ${src}`);
  process.exit(1);
}

fs.mkdirSync(dest, { recursive: true });
fs.cpSync(src, dest, { recursive: true, force: true });
console.log(`k3ctx skill installed → ${dest}`);
