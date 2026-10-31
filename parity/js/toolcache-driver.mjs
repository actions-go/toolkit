// Runs tool-cache parity scenarios against the reference @actions/tool-cache
// and prints the observed behaviour as JSON. The Go side
// (parity/cache/parity_test.go) produces the same shape for the same scenarios
// and the test compares the two.
//
// Each scenario runs in its own <root>/{src,cache,temp} directories, with
// RUNNER_TOOL_CACHE=<root>/cache and RUNNER_TEMP=<root>/temp.
//
// usage: node toolcache-driver.mjs <scenarios.json> <server base url>
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import * as tc from '@actions/tool-cache'

// downloadTool retries three times, sleeping 10-20s between attempts; the
// reference exposes these globals so tests do not wait.
globalThis.TEST_DOWNLOAD_TOOL_RETRY_MIN_SECONDS = 0
globalThis.TEST_DOWNLOAD_TOOL_RETRY_MAX_SECONDS = 0

const scenarios = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'))
const server = process.argv[3]
const results = {}

for (const sc of scenarios) {
  results[sc.name] = await run(sc)
}
process.stdout.write(JSON.stringify(results))

// materialise creates the fixture entries below base.
function materialise(base, entries) {
  fs.mkdirSync(base, { recursive: true })
  for (const e of entries || []) {
    const p = path.join(base, e.path)
    if (e.symlink !== undefined) {
      fs.mkdirSync(path.dirname(p), { recursive: true })
      fs.symlinkSync(e.symlink, p)
    } else if (e.dir) {
      fs.mkdirSync(p, { recursive: true })
    } else {
      fs.mkdirSync(path.dirname(p), { recursive: true })
      fs.writeFileSync(p, e.content || '')
    }
    if (e.mode && e.symlink === undefined) fs.chmodSync(p, parseInt(e.mode, 8))
  }
}

// snapshot describes every entry below root with root-relative paths.
function snapshot(root) {
  const tree = {}
  const walk = dir => {
    for (const name of fs.readdirSync(dir).sort()) {
      const p = path.join(dir, name)
      const rel = path.relative(root, p).split(path.sep).join('/')
      const st = fs.lstatSync(p)
      // Windows has no Unix permission bits: modes are not compared there
      const mode = process.platform === 'win32' ? undefined : '0' + (st.mode & 0o777).toString(8)
      if (st.isSymbolicLink()) {
        tree[rel] = { kind: 'symlink', target: fs.readlinkSync(p) }
      } else if (st.isDirectory()) {
        tree[rel] = { kind: 'dir', mode }
        walk(p)
      } else {
        const data = fs.readFileSync(p)
        const text = data.toString('utf8')
        const content = Buffer.from(text, 'utf8').equals(data) ? text : 'base64:' + data.toString('base64')
        tree[rel] = { kind: 'file', mode, content }
      }
    }
  }
  walk(root)
  return tree
}

async function run(sc) {
  for (const k of Object.keys(process.env)) {
    if (/^(RUNNER_|ACTIONS_|GITHUB_|INPUT_)/.test(k)) delete process.env[k]
  }
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'parity-toolcache-js-'))
  const dirs = { src: path.join(root, 'src'), cache: path.join(root, 'cache'), temp: path.join(root, 'temp') }
  materialise(dirs.src, sc.src)
  materialise(dirs.cache, sc.cache)
  materialise(dirs.temp, sc.temp)
  process.env.RUNNER_TOOL_CACHE = dirs.cache
  process.env.RUNNER_TEMP = dirs.temp
  for (const [k, v] of Object.entries(sc.env || {})) process.env[k] = v

  const subst = v =>
    typeof v === 'string'
      ? v
          .replaceAll('{{src}}', dirs.src)
          .replaceAll('{{cache}}', dirs.cache)
          .replaceAll('{{temp}}', dirs.temp)
          .replaceAll('{{server}}', server)
      : v

  let stdout = ''
  const realWrite = process.stdout.write
  process.stdout.write = chunk => {
    stdout += chunk
    return true
  }
  const returns = []
  try {
    for (const call of sc.calls) {
      try {
        const value = await tc[call.fn](...(call.args || []).map(subst))
        returns.push({ value: value === undefined ? null : value })
      } catch (e) {
        returns.push({ error: e.message })
      }
    }
  } finally {
    process.stdout.write = realWrite
  }

  const out = { root, stdout, returns, tree: snapshot(root) }
  fs.rmSync(root, { recursive: true, force: true })
  return out
}
