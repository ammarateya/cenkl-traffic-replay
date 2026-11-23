import { createServer } from 'node:http';
import { execFileSync } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { JobQueue } from './jobs.js';

const serviceDir = dirname(fileURLToPath(import.meta.url));
const root = resolve(serviceDir, '..');
const storage = join(root, 'work', 'service-jobs');
const queue = new JobQueue(root, storage, async () => {
  const { Codex } = await import('@openai/codex-sdk');
  let cli = process.env.CENKL_CODEX_BIN;
  if (!cli) {
    try { cli = execFileSync('which', ['codex'], { encoding: 'utf8' }).trim(); } catch { throw new Error('Set CENKL_CODEX_BIN to an installed Codex CLI'); }
  }
  return new Codex({ codexPathOverride: join(serviceDir, 'codex-local.sh'), env: { ...process.env, CENKL_CODEX_REAL_BIN: cli } });
});
await queue.init();

function send(res, status, data, type = 'application/json') {
  res.writeHead(status, { 'content-type': `${type}; charset=utf-8`, 'cache-control': 'no-store', 'x-content-type-options': 'nosniff' });
  res.end(type === 'application/json' ? JSON.stringify(data) : data);
}

export function makeServer(jobQueue = queue) {
  return createServer(async (req, res) => {
    try {
      const path = new URL(req.url, 'http://localhost').pathname;
      if (req.method === 'GET' && path === '/') return send(res, 200, await readFile(join(serviceDir, 'index.html'), 'utf8'), 'text/html');
      if (req.method === 'POST' && path === '/api/jobs') {
        let body = '';
        for await (const chunk of req) {
          body += chunk;
          if (body.length > 4096) return send(res, 413, { error: 'Request too large' });
        }
        const job = await jobQueue.submit(JSON.parse(body));
        return send(res, 202, job);
      }
      const match = path.match(/^\/api\/jobs\/([0-9a-f-]{36})$/);
      if (req.method === 'GET' && match) return send(res, 200, await jobQueue.get(match[1]));
      const reportMatch = path.match(/^\/api\/jobs\/([0-9a-f-]{36})\/report$/);
      if (req.method === 'GET' && reportMatch) {
        const job = await jobQueue.get(reportMatch[1]);
        if (job.status !== 'analyzed') return send(res, 404, { error: 'Report is not ready' });
        return send(res, 200, JSON.parse(await readFile(join(jobQueue.storage, job.id, 'report.json'), 'utf8')));
      }
      const candidateMatch = path.match(/^\/api\/jobs\/([0-9a-f-]{36})\/candidate$/);
      if (req.method === 'GET' && candidateMatch) {
        const job = await jobQueue.get(candidateMatch[1]);
        if (job.status !== 'needs_review') return send(res, 404, { error: 'Candidate is not ready' });
        return send(res, 200, await readFile(join(jobQueue.storage, job.id, 'repo', 'SETUP_CANDIDATE.md'), 'utf8'), 'text/plain');
      }
      return send(res, 404, { error: 'Not found' });
    } catch (error) {
      const badInput = error instanceof SyntaxError || /GitHub|Choose|Invalid/.test(String(error.message));
      return send(res, badInput ? 400 : 500, { error: String(error.message) });
    }
  });
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const host = process.env.CENKL_SERVICE_HOST || '127.0.0.1';
  const port = Number(process.env.CENKL_SERVICE_PORT || 8081);
  makeServer().listen(port, host, () => console.log(`CenKL setup service: http://${host}:${port}`));
}
