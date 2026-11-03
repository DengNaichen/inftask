import { execFile, execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import https from 'node:https';
import { X509Certificate } from 'node:crypto';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

const require = createRequire(import.meta.url);
const electron = require('electron');
const temporary = mkdtempSync(path.join(tmpdir(), 'inftask-desktop-smoke-'));
const script = fileURLToPath(new URL('smoke-app.cjs', import.meta.url));
const output = process.argv.slice(2).find((arg) => arg !== '--cloud');
const evidence = output ? path.resolve(output) : temporary;
mkdirSync(evidence, { recursive: true });
const servers = [];
const env = { ...process.env };
delete env.ELECTRON_RUN_AS_NODE;
const run = async (args) => {
  const { stdout, stderr } = await promisify(execFile)(electron, [script, temporary, evidence, ...args],
    { env, timeout: 90_000, maxBuffer: 1024 * 1024 });
  process.stdout.write(stdout);
  process.stderr.write(stderr);
};
try {
  if (process.argv.includes('--cloud')) {
    await run(['cloud']);
  } else {
    const key = path.join(temporary, 'key.pem');
    const cert = path.join(temporary, 'cert.pem');
    execFileSync('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes',
      '-keyout', key, '-out', cert, '-days', '1', '-subj', '/CN=127.0.0.1'],
      { stdio: 'ignore', timeout: 20_000 });
    const options = { key: readFileSync(key), cert: readFileSync(cert) };
    const fingerprint = new X509Certificate(options.cert).fingerprint256;
    const listen = async (handler) => {
      const server = https.createServer(options, handler);
      servers.push(server);
      await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
      return 'https://127.0.0.1:' + server.address().port;
    };
    let cloud;
    let cloudServer;
    let cloudPort;
    const auth = await listen((req, res) => {
      if (req.url === '/stop-cloud') {
        cloudServer.closeAllConnections();
        cloudServer.close(() => res.end('stopped'));
        return;
      }
      if (req.url === '/start-cloud') {
        cloudServer.listen(cloudPort, '127.0.0.1', () => res.end('started'));
        return;
      }
      res.setHeader('Content-Type', 'text/html');
      res.end('<h1>Fixture sign in</h1><a id="sign-in" href="' + cloud + '/callback">Sign in with fake identity</a>');
    });
    cloud = await listen((req, res) => {
      res.setHeader('Cache-Control', 'no-store');
      if (req.url === '/callback') {
        res.setHeader('Set-Cookie', 'fixture_session=synthetic; Path=/; Secure; HttpOnly; SameSite=Lax; Max-Age=3600');
        res.writeHead(302, { Location: '/' }).end();
      } else if (req.url === '/auth/login' || !req.headers.cookie?.includes('fixture_session=synthetic')) {
        res.writeHead(302, { Location: auth + '/authorize' }).end();
      } else if (req.url === '/unavailable') {
        res.writeHead(503).end('Fixture temporarily unavailable');
      } else if (req.url === '/bad-redirect') {
        res.writeHead(302, { Location: 'https://evil.invalid/' }).end();
      } else {
        res.setHeader('Content-Type', 'text/html');
        res.end('<h1 id="signed-in">Fixture workspace</h1><textarea id="draft"></textarea>' +
          '<a id="external" href="https://github.com/DengNaichen/inftask">GitHub</a>');
      }
    });
    cloudServer = servers.at(-1);
    cloudPort = cloudServer.address().port;
    await run(['first', cloud + '/', auth, fingerprint]);
    await run(['second', cloud + '/', auth, fingerprint]);
    console.log('PASS: HTTPS fixture login, persistent cookie across processes, hardened windows, navigation, Dock reopen and failure/retry.');
  }
} finally {
  for (const server of servers) await new Promise((resolve) => server.close(resolve));
  rmSync(temporary, { recursive: true });
}
