// Runs parity scenarios against the reference @actions/core and prints the
// observed behaviour as JSON. The Go side (parity/parity_test.go) produces the
// same shape for the same scenarios and the test compares the two.
//
// usage: node driver.mjs <scenarios.json>
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import * as core from '@actions/core'

const FILE_ENVS = {
  output: 'GITHUB_OUTPUT',
  env: 'GITHUB_ENV',
  path: 'GITHUB_PATH',
  state: 'GITHUB_STATE',
  summary: 'GITHUB_STEP_SUMMARY'
}

const scenarios = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'))
const results = {}

for (const sc of scenarios) {
  results[sc.name] = await run(sc)
}
// core.setFailed sets process.exitCode = 1; the exit status is not part of what
// is compared (the Go package keeps it private), so do not let it fail the driver.
process.exitCode = 0
process.stdout.write(JSON.stringify(results))

async function run(sc) {
  // start from a clean environment so scenarios cannot leak into each other
  for (const k of Object.keys(process.env)) {
    if (/^(INPUT_|STATE_|GITHUB_|RUNNER_|ACTIONS_)/.test(k) || k === 'PATH') {
      delete process.env[k]
    }
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'parity-js-'))
  const files = {}
  if (!sc.noFiles) {
    for (const [key, envName] of Object.entries(FILE_ENVS)) {
      files[key] = path.join(dir, key)
      fs.writeFileSync(files[key], '')
      process.env[envName] = files[key]
    }
  }
  for (const [k, v] of Object.entries(sc.env || {})) process.env[k] = v

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
        const value = await core[call.fn](...(call.args || []))
        returns.push({ value: value === undefined ? null : value })
      } catch (e) {
        returns.push({ error: e.message })
      }
    }
  } finally {
    process.stdout.write = realWrite
  }

  const out = { stdout, returns, files: {}, env: {} }
  for (const key of Object.keys(files)) {
    out.files[key] = fs.readFileSync(files[key], 'utf8')
  }
  for (const k of sc.observeEnv || []) out.env[k] = process.env[k] ?? null
  fs.rmSync(dir, { recursive: true, force: true })
  return out
}
