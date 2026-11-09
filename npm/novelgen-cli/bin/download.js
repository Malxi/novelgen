'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const https = require('https');

const pkg = require('../package.json');
const version = process.env.NOVELGEN_VERSION || pkg.version;
const repo = 'Malxi/novelgen';

function platformAsset() {
  const platform = process.platform;
  const arch = process.arch;
  const key = `${platform}-${arch}`;
  const assets = {
    'win32-x64': 'novelgen-windows-amd64.exe',
    'win32-arm64': 'novelgen-windows-arm64.exe',
    'darwin-x64': 'novelgen-darwin-amd64',
    'darwin-arm64': 'novelgen-darwin-arm64',
    'linux-x64': 'novelgen-linux-amd64',
    'linux-arm64': 'novelgen-linux-arm64',
  };
  const asset = assets[key];
  if (!asset) {
    throw new Error(`Novelgen does not publish a binary for ${key}`);
  }
  return asset;
}

function cacheDir() {
  return path.join(os.homedir(), '.novelgen', 'bin', version);
}

function binaryPath() {
  if (process.env.NOVELGEN_BIN) {
    return process.env.NOVELGEN_BIN;
  }
  return path.join(cacheDir(), platformAsset());
}

function download(url, destination, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (redirects > 8) {
      reject(new Error('too many redirects'));
      return;
    }
    https.get(url, { headers: { 'user-agent': 'novelgen-cli' } }, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        resolve(download(response.headers.location, destination, redirects + 1));
        return;
      }
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`download failed: HTTP ${response.statusCode} ${url}`));
        return;
      }
      const temp = `${destination}.download-${process.pid}`;
      const output = fs.createWriteStream(temp);
      response.pipe(output);
      output.on('finish', () => {
        output.close(() => {
          try {
            fs.renameSync(temp, destination);
            if (process.platform !== 'win32') {
              fs.chmodSync(destination, 0o755);
            }
            resolve(destination);
          } catch (error) {
            reject(error);
          }
        });
      });
      output.on('error', (error) => {
        fs.rm(temp, { force: true }, () => reject(error));
      });
    }).on('error', reject);
  });
}

async function ensureBinary() {
  const destination = binaryPath();
  if (fs.existsSync(destination)) {
    return destination;
  }
  if (process.env.NOVELGEN_BIN) {
    throw new Error(`NOVELGEN_BIN points to a missing file: ${destination}`);
  }
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  const asset = platformAsset();
  const url = `https://github.com/${repo}/releases/download/v${version}/${asset}`;
  process.stderr.write(`novelgen: downloading ${asset} ${version}\n`);
  await download(url, destination);
  return destination;
}

module.exports = { ensureBinary, binaryPath, platformAsset, version };
