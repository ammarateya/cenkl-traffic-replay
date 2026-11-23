import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { JobQueue, parseGithubUrl, validateSubmission } from './jobs.js';

test('GitHub URL validation rejects credentials and extra paths', () => {
  assert.equal(parseGithubUrl('https://github.com/a/b'), 'https://github.com/a/b.git');
  for (const url of ['https://user:pass@github.com/a/b', 'https://github.com/a/b/tree/main', 'http://github.com/a/b', 'https://example.com/a/b']) assert.throws(() => parseGithubUrl(url));
  assert.deepEqual(validateSubmission({ type: 'preset', preset: 'gost' }), { type: 'preset', preset: 'gost', sample: 'smoke' });
});

test('queue persists a completed job', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'cenkl-jobs-'));
  const queue = new JobQueue(dir, join(dir, 'jobs'), async () => { throw Error('unused'); });
  await queue.init();
  queue.runPreset = async job => { job.status = 'smoke_passed'; job.runDirectory = 'work/example'; };
  const created = await queue.submit({ type: 'preset', preset: 'gost' });
  while (queue.busy || queue.pending.length) await new Promise(r => setTimeout(r, 5));
  const saved = JSON.parse(await readFile(join(dir, 'jobs', created.id, 'job.json')));
  assert.equal(saved.status, 'smoke_passed');
  assert.equal(saved.runDirectory, 'work/example');
});
