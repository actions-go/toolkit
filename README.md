
<!--
<p align="center">
  <a href="https://github.com/actions-go/toolkit"><img alt="GitHub Actions status" src="https://github.com/actions-go/toolkit/workflows/ci/badge.svg"></a>
</p>
-->

[![GoDoc](https://godoc.org/github.com/actions-go/toolkit?status.svg)](https://godoc.org/github.com/actions-go/toolkit)

## GitHub Actions Go (aka golang) Toolkit

The GitHub Actions Go ToolKit provides a set of packages to make creating actions easier.

This toolkit is a pure go port of the official [@actions/toolkit](https://github.com/actions/toolkit)

<br/>
<h3 align="center">Get started with the <a href="https://github.com/tjamet/go-action-template">go-action-template</a>!</h3>
<br/>

## Packages

:heavy_check_mark: [github.com/actions-go/core](core) 

[![GoDoc](https://godoc.org/github.com/actions-go/toolkit/core?status.svg)](https://godoc.org/github.com/actions-go/toolkit/core)

Provides functions for inputs, outputs, results, logging, secrets and variables. Read more [here](https://godoc.org/github.com/actions-go/toolkit/core)

```bash
$ go get github.com/actions-go/core
```
<br/>

:hammer: [github.com/actions-go/cache](cache) 

[![GoDoc](https://godoc.org/github.com/actions-go/toolkit/cache?status.svg)](https://godoc.org/github.com/actions-go/toolkit/cache)

Provides functions for downloading and caching tools.  e.g. setup-* actions. Read more [here](https://godoc.org/github.com/actions-go/toolkit/cache)

```bash
$ go get github.com/actions-go/cache
```
<br/>

:octocat: [github.com/actions-go/github](github) 

[![GoDoc](https://godoc.org/github.com/actions-go/toolkit/github?status.svg)](https://godoc.org/github.com/actions-go/toolkit/github)

Provides an authenticated GitHub client hydrated with the context that the current action is being run in. Read more [here](https://godoc.org/github.com/actions-go/toolkit/github)

```bash
$ go get github.com/actions-go/github
```
<br/>

## Creating an Action with the Toolkit

:question: [Choosing an action type](https://github.com/actions/toolkit/docs/action-types.md)

Outlines the differences and why you would want to create a JavaScript or a container based action.

<br/>
<br/>

:curly_loop: [Versioning](https://github.com/actions/toolkit/docs/action-versioning.md)

Actions are downloaded and run from the GitHub graph of repos.  This contains guidance for versioning actions and safe releases.
<br/>
<br/>

:warning: [Problem Matchers](https://github.com/actions/toolkit/docs/problem-matchers.md)

Problem Matchers are a way to scan the output of actions for a specified regex pattern and surface that information prominently in the UI.
<br/>
<br/>

<h3><a href="https://github.com/actions-go/hello-world">Hello World Go Action</a></h3>


Illustrates how to create a simple hello world javascript action.

```go
import "github.com/actions-go/toolkit/core"

func main() {
    whoToGreet := core.GetInput("who-to-greet")
    fmt.Println("Hello", whoToGreet)
}
```

<h3><a href="https://github.com/actions/hello-world-javascript-action">Hello World JavaScript Action</a></h3>

Illustrates how to create a simple hello world javascript action.

```javascript
...
  const nameToGreet = core.getInput('who-to-greet');
  console.log(`Hello ${nameToGreet}!`);
...
```
<br/>

<h3><a href="https://github.com/actions/javascript-action">JavaScript Action Walkthrough</a></h3>
 
Walkthrough and template for creating a JavaScript Action with tests, linting, workflow, publishing, and versioning.

```javascript
async function run() {
  try { 
    const ms = core.getInput('milliseconds');
    console.log(`Waiting ${ms} milliseconds ...`)
    ...
```
```javascript
PASS ./index.test.js
  ✓ throws invalid number 
  ✓ wait 500 ms 
  ✓ test runs

Test Suites: 1 passed, 1 total    
Tests:       3 passed, 3 total
```
<br/>

<h3><a href="https://github.com/actions/typescript-action">TypeScript Action Walkthrough</a></h3>

Walkthrough creating a TypeScript Action with compilation, tests, linting, workflow, publishing, and versioning.

```javascript
import * as core from '@actions/core';

async function run() {
  try {
    const ms = core.getInput('milliseconds');
    console.log(`Waiting ${ms} milliseconds ...`)
    ...
```
```javascript
PASS ./index.test.js
  ✓ throws invalid number 
  ✓ wait 500 ms 
  ✓ test runs

Test Suites: 1 passed, 1 total    
Tests:       3 passed, 3 total
```
<br/>
<br/>

<h3><a href="https://github.com/actions/toolkit/docs/container-action.md">Docker Action Walkthrough</a></h3>

Create an action that is delivered as a container and run with docker.

```docker
FROM alpine:3.10
COPY LICENSE README.md /
COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
```
<br/>

<h3><a href="https://github.com/actions/container-toolkit-action">Docker Action Walkthrough with Octokit</a></h3>

Create an action that is delivered as a container which uses the toolkit.  This example uses the GitHub context to construct an Octokit client.

```docker
FROM node:slim
COPY . .
RUN npm install --production
ENTRYPOINT ["node", "/lib/main.js"]
```
```javascript
const myInput = core.getInput('myInput');
core.debug(`Hello ${myInput} from inside a container`);

const context = github.context;
console.log(`We can even get context data, like the repo: ${context.repo.repo}`)    
```
<br/>

<!--
## Contributing

We welcome contributions.  See [how to contribute](docs/contribute.md).

## Code of Conduct

See [our code of conduct](CODE_OF_CONDUCT.md).
-->