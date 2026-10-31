// Runs parity scenarios against the reference @actions/github and prints the
// observed behaviour as JSON. The Go side (parity/github/github_test.go)
// produces the same shape for the same scenarios and the test compares the two.
//
// usage: node github-driver.mjs <scenarios.json> <server-url>
//
// <server-url> is a local HTTP server owned by the Go test; request scenarios
// send their request there and the Go test records it.
//
// @actions/github reads GITHUB_API_URL once, when the module is imported, so the
// baseUrl and request scenarios each run in a child process of this driver
// (`--one <scenario.json> <server-url>`) with that scenario's environment.
//
// Go zero values stand in for JS "unset": undefined strings become "" and NaN
// numbers become 0, because the Go Context cannot represent either.
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const oneMode = process.argv[2] === '--one'
const serverUrl = (oneMode ? process.argv[4] : process.argv[3]) || ''
const scenarios = JSON.parse(fs.readFileSync(oneMode ? process.argv[3] : process.argv[2], 'utf8'))
const scenarioList = oneMode ? [scenarios] : scenarios

const tmpDirs = []
setup(scenarioList[0])
// imported only now: @actions/github builds its Context and base URL at import time
const github = await import('@actions/github')
const { GitHub, getUserAgentWithOrchestrationId } = await import('@actions/github/lib/utils')
const Context = github.context.constructor

if (oneMode) {
  const res = await run(scenarios)
  cleanup()
  process.stdout.write(JSON.stringify(res))
} else {
  const results = {}
  for (const sc of scenarioList) {
    results[sc.name] = needsFreshProcess(sc) ? runInChild(sc) : await run(sc)
  }
  cleanup()
  process.stdout.write(JSON.stringify(results))
}

function cleanup() {
  for (const d of tmpDirs) fs.rmSync(d, { recursive: true, force: true })
}

function needsFreshProcess(sc) {
  return sc.kind === 'baseUrl' || sc.kind === 'request'
}

function runInChild(sc) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'parity-js-github-child-'))
  tmpDirs.push(dir)
  const file = path.join(dir, 'scenario.json')
  fs.writeFileSync(file, JSON.stringify(sc))
  const child = spawnSync(process.execPath, [fileURLToPath(import.meta.url), '--one', file, serverUrl], {
    encoding: 'utf8',
    env: process.env
  })
  if (child.status !== 0) throw new Error(`child for ${sc.name} failed: ${child.stderr}`)
  return JSON.parse(child.stdout)
}

// setup starts from a clean environment, so scenarios cannot leak into each
// other, and applies the scenario's.
function setup(sc) {
  for (const k of Object.keys(process.env)) {
    if (/^(INPUT_|STATE_|GITHUB_|RUNNER_|ACTIONS_)/.test(k)) delete process.env[k]
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'parity-js-github-'))
  tmpDirs.push(dir)
  if (sc.event !== undefined || sc.eventRaw !== undefined) {
    const file = path.join(dir, 'event.json')
    fs.writeFileSync(file, sc.eventRaw ?? JSON.stringify(sc.event))
    process.env.GITHUB_EVENT_PATH = file
  }
  for (const [k, v] of Object.entries(sc.env || {})) {
    // the scenario name prefixes the path so the Go test can tell which scenario sent a request
    process.env[k] = v.replaceAll('{{SERVER}}', `${serverUrl}/${encodeURIComponent(sc.name)}`)
  }
}

async function run(sc) {
  setup(sc)
  switch (sc.kind) {
    case 'context':
      return runContext(sc)
    case 'userAgent':
      return { value: getUserAgentWithOrchestrationId(sc.base ?? undefined) ?? '' }
    case 'baseUrl':
      return { value: new GitHub().request.endpoint.DEFAULTS.baseUrl }
    case 'request':
      return await runRequest(sc)
    default:
      throw new Error(`unknown scenario kind ${sc.kind}`)
  }
}

function runContext(sc) {
  let stdout = ''
  const realWrite = process.stdout.write
  process.stdout.write = chunk => {
    stdout += chunk
    return true
  }
  let ctx
  const out = {}
  try {
    ctx = new Context()
  } catch (e) {
    out.constructorError = e.message
  } finally {
    process.stdout.write = realWrite
  }
  out.stdout = stdout
  if (!ctx) return out

  const str = v => v ?? ''
  const num = v => (Number.isNaN(v) ? 0 : v)
  out.fields = {
    eventName: str(ctx.eventName),
    sha: str(ctx.sha),
    ref: str(ctx.ref),
    workflow: str(ctx.workflow),
    action: str(ctx.action),
    actor: str(ctx.actor),
    job: str(ctx.job),
    runNumber: num(ctx.runNumber),
    runId: num(ctx.runId),
    runAttempt: num(ctx.runAttempt),
    apiUrl: ctx.apiUrl,
    serverUrl: ctx.serverUrl,
    graphqlUrl: ctx.graphqlUrl
  }
  out.repo = capture(() => ({ owner: str(ctx.repo.owner), repo: str(ctx.repo.repo) }))
  out.issue = capture(() => ({ owner: str(ctx.issue.owner), repo: str(ctx.issue.repo), number: ctx.issue.number ?? 0 }))
  out.payload = {}
  for (const p of sc.payloadPaths || []) out.payload[p] = lookup(ctx.payload, p)
  return out
}

// capture mirrors the Go side: a getter that throws is recorded as an error.
function capture(fn) {
  try {
    return { value: fn() }
  } catch (e) {
    return { error: e.message }
  }
}

function lookup(obj, dotted) {
  let cur = obj
  for (const part of dotted.split('.')) {
    if (cur === null || cur === undefined) return null
    cur = cur[part]
  }
  return cur ?? null
}

async function runRequest(sc) {
  const p = sc.params || {}
  try {
    const octokit = github.getOctokit(sc.token || undefined)
    let res
    switch (sc.call) {
      case 'repos.get':
        res = await octokit.rest.repos.get({ owner: p.owner, repo: p.repo })
        break
      case 'issues.createComment':
        res = await octokit.rest.issues.createComment({ owner: p.owner, repo: p.repo, issue_number: p.number, body: p.body })
        break
      default:
        throw new Error(`unknown call ${sc.call}`)
    }
    return { status: res.status }
  } catch (e) {
    return { error: e.message }
  }
}
