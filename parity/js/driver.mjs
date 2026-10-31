// Runs parity scenarios against the reference @actions/core and prints the
// observed behaviour as JSON. The Go side (parity/parity_test.go) produces the
// same shape for the same scenarios and the test compares the two.
//
// usage: node driver.mjs <scenarios.json>
//
// Scenario format (all fields except name and calls are optional):
//   name        unique scenario name
//   env         environment variables to set; "{{server}}" is replaced by the
//               base URL of the scenario's local http server
//   inheritEnv  names copied from the real process environment (e.g. PATH)
//   noFiles     do not create the GITHUB_* file command files
//   observeEnv  environment variables whose final value is reported
//   server      {responses: [{status, body, headers}]}: a local http server
//               answering requests with the responses in order (the last one
//               repeats); the requests it received are reported
//   calls       [{fn, args}] executed in order. fn is a member path of
//               @actions/core: "setOutput", "summary.addRaw", "platform.arch".
//               An argument {"$calls": [...], "throw": "msg"} is a callback that
//               runs the nested calls (and then throws msg, if given).
//               Builder methods that return the summary report "<summary>".
import fs from 'node:fs'
import http from 'node:http'
import os from 'node:os'
import path from 'node:path'
import * as core from '@actions/core'
// The reference exposes no stop-commands helper; its internal command.issue is the
// primitive the runner protocol is documented against, so it is the reference here.
import { issue } from './node_modules/@actions/core/lib/command.js'

const FILE_ENVS = {
  output: 'GITHUB_OUTPUT',
  env: 'GITHUB_ENV',
  path: 'GITHUB_PATH',
  state: 'GITHUB_STATE',
  summary: 'GITHUB_STEP_SUMMARY'
}

// Reference-side shims for Go helpers the reference does not export.
const shims = {
  stopCommands: endToken => issue('stop-commands', endToken),
  startCommands: endToken => issue(endToken)
}

const RECORDED_HEADERS = ['authorization', 'accept', 'content-type', 'user-agent']

// scenarios clear parts of the environment, inheritEnv copies from the original
const originalEnv = { ...process.env }
const scenarios = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'))
const results = {}

for (const sc of scenarios) {
  results[sc.name] = await run(sc)
}
// run() resets process.exitCode after recording it; make sure the driver itself exits 0
process.exitCode = 0
process.stdout.write(JSON.stringify(results))

async function startServer(spec) {
  const requests = []
  let n = 0
  const server = http.createServer((req, res) => {
    const headers = {}
    for (const h of RECORDED_HEADERS) {
      if (req.headers[h] !== undefined) headers[h] = req.headers[h]
    }
    requests.push({ method: req.method, url: req.url, headers })
    const r = spec.responses[Math.min(n++, spec.responses.length - 1)]
    res.writeHead(r.status, r.headers || {})
    res.end(r.body || '')
  })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  return { server, requests, url: `http://127.0.0.1:${server.address().port}` }
}

function callable(spec) {
  return async () => {
    await execAll(spec.$calls)
    if (spec.throw) throw new Error(spec.throw)
    return spec.returns
  }
}

function prepareArgs(args) {
  return (args || []).map(a =>
    a !== null && typeof a === 'object' && !Array.isArray(a) && '$calls' in a ? callable(a) : a
  )
}

async function execAll(calls) {
  const returns = []
  for (const call of calls) {
    returns.push(await exec(call))
  }
  return returns
}

async function exec(call) {
  try {
    const args = prepareArgs(call.args)
    let value
    if (shims[call.fn]) {
      value = await shims[call.fn](...args)
    } else {
      const parts = call.fn.split('.')
      let owner = core
      for (const p of parts.slice(0, -1)) owner = owner[p]
      const member = owner[parts[parts.length - 1]]
      value = typeof member === 'function' ? await member.apply(owner, args) : member
    }
    if (value === core.summary) value = '<summary>'
    return { value: value === undefined ? null : value }
  } catch (e) {
    return { error: e.message }
  }
}

async function run(sc) {
  // start from a clean environment so scenarios cannot leak into each other
  const inherited = {}
  for (const k of sc.inheritEnv || []) inherited[k] = originalEnv[k]
  for (const k of Object.keys(process.env)) {
    if (/^(INPUT_|STATE_|GITHUB_|RUNNER_|ACTIONS_)/.test(k) || k === 'PATH') {
      delete process.env[k]
    }
  }
  for (const [k, v] of Object.entries(inherited)) if (v !== undefined) process.env[k] = v
  // the summary builder caches the file path and buffer for the whole process
  core.summary._filePath = undefined
  core.summary.emptyBuffer()

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'parity-js-'))
  const files = {}
  if (!sc.noFiles) {
    for (const [key, envName] of Object.entries(FILE_ENVS)) {
      files[key] = path.join(dir, key)
      fs.writeFileSync(files[key], '')
      process.env[envName] = files[key]
    }
  }
  const srv = sc.server ? await startServer(sc.server) : null
  for (const [k, v] of Object.entries(sc.env || {})) {
    process.env[k] = v.replaceAll('{{server}}', srv ? srv.url : '')
  }

  let stdout = ''
  const realWrite = process.stdout.write
  process.stdout.write = chunk => {
    stdout += chunk
    return true
  }
  let returns
  try {
    returns = await execAll(sc.calls)
  } finally {
    process.stdout.write = realWrite
  }

  // core.setFailed sets process.exitCode = 1
  const out = { stdout, returns, failed: process.exitCode === 1, files: {}, env: {} }
  process.exitCode = 0
  for (const key of Object.keys(files)) {
    out.files[key] = fs.readFileSync(files[key], 'utf8')
  }
  for (const k of sc.observeEnv || []) out.env[k] = process.env[k] ?? null
  if (srv) {
    out.requests = srv.requests
    srv.server.closeAllConnections()
    await new Promise(resolve => srv.server.close(resolve))
  }
  fs.rmSync(dir, { recursive: true, force: true })
  return out
}
