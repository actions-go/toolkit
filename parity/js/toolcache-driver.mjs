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
import http from 'node:http'
import https from 'node:https'
import { syncBuiltinESMExports } from 'node:module'
import * as tc from '@actions/tool-cache'
// the internal helpers of the reference (_getOsVersion...) are not exported by the
// package, the file is imported by its location
const mm = await import(new URL('./node_modules/@actions/tool-cache/lib/manifest.js', import.meta.url))

// downloadTool retries three times, sleeping 10-20s between attempts; the
// reference exposes these globals so tests do not wait.
globalThis.TEST_DOWNLOAD_TOOL_RETRY_MIN_SECONDS = 0
globalThis.TEST_DOWNLOAD_TOOL_RETRY_MAX_SECONDS = 0

const scenarios = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'))
const server = process.argv[3]
const results = {}

// getManifestFromRepo reads https://api.github.com, which the reference does not
// let anyone change. The requests to that host are sent to the test server
// instead (the Go side points cache.ManifestAPIURL at it).
const serverURL = new URL(server)
const httpsRequest = https.request
https.request = (options, ...rest) => {
  if (options && options.host === 'api.github.com') {
    return http.request({ ...options, host: serverURL.hostname, port: serverURL.port, agent: undefined }, ...rest)
  }
  return httpsRequest(options, ...rest)
}
// @actions/http-client keeps the first agent it creates and hands it to every
// later request. After the api.github.com request that agent is an https one,
// and the follow-up requests (blob and redirect URLs on the test server, which
// are plain http) would be refused with ERR_INVALID_PROTOCOL. With a proxy
// variable set the client takes another branch, which hid this on a developer
// machine. Real GitHub only answers with https URLs, so drop the https agent
// from plain http requests.
const httpRequest = http.request
http.request = (options, ...rest) => {
  if (options && typeof options === 'object' && options.agent && options.agent.protocol === 'https:') {
    options = { ...options, agent: undefined }
  }
  return httpRequest(options, ...rest)
}
syncBuiltinESMExports()

for (const sc of scenarios) {
  results[sc.name] = await run(sc)
}
// a request that failed halfway can leave a socket open, which would keep the process alive
process.stdout.write(JSON.stringify(results), () => process.exit(0))

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
      fs.writeFileSync(p, e.base64 ? Buffer.from(e.base64, 'base64') : e.content || '')
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

  const subst = v => {
    if (Array.isArray(v)) return v.map(subst)
    if (v && typeof v === 'object') return Object.fromEntries(Object.entries(v).map(([k, x]) => [k, subst(x)]))
    if (typeof v !== 'string') return v
    return v
      .replaceAll('{{src}}', dirs.src)
      .replaceAll('{{cache}}', dirs.cache)
      .replaceAll('{{temp}}', dirs.temp)
      .replaceAll('{{server}}', server)
      .replaceAll('{{osversion}}', v.includes('{{osversion}}') ? mm._getOsVersion() : '')
  }

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
        const fn = call.fn.startsWith('_') ? mm[call.fn] : tc[call.fn]
        const value = await fn(...(call.args || []).map(subst))
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
