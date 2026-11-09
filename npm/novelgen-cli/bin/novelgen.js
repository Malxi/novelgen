#!/usr/bin/env node
'use strict';

const { spawn } = require('child_process');
const { ensureBinary } = require('./download');

async function main() {
  const binary = await ensureBinary();
  const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit' });
  child.on('error', (error) => {
    console.error(`novelgen: ${error.message}`);
    process.exit(1);
  });
  child.on('exit', (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code === null ? 1 : code);
  });
}

main().catch((error) => {
  console.error(`novelgen: ${error.message}`);
  process.exit(1);
});
