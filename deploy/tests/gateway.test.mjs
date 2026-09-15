import assert from 'node:assert/strict'
import { once } from 'node:events'
import { createServer } from 'node:http'
import { setTimeout as delay } from 'node:timers/promises'
import test from 'node:test'

const baseURL = 'http://gateway'

test('same-origin gateway', { timeout: 60_000 }, async (t) => {
  let canceledStream = false
  const download = Buffer.from('report,amount\nfixture,123.45\n'.repeat(4096))
  const backend = createServer(async (request, response) => {
    response.setHeader('X-Upstream', 'backend')

    if (request.url === '/api/v1/stream' || request.url === '/api/v1/stream/cancel') {
      response.writeHead(200, {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache',
      })
      response.write('data: first\n\n')
      const finish = setTimeout(() => response.end('data: last\n\n'), 3000)
      response.on('close', () => {
        clearTimeout(finish)
        if (request.url.endsWith('/cancel')) canceledStream = true
      })
      return
    }

    if (request.url === '/api/v1/download') {
      response.writeHead(200, {
        'Content-Type': 'text/csv',
        'Content-Disposition': 'attachment; filename="report.csv"',
      })
      response.end(download)
      return
    }

    if (request.url === '/api/v1/unavailable') {
      response.writeHead(503, { 'Content-Type': 'application/json', 'Retry-After': '10' })
      response.end(JSON.stringify({ error: 'upstream unavailable' }))
      return
    }

    const chunks = []
    for await (const chunk of request) chunks.push(chunk)
    const body = Buffer.concat(chunks)
    response.setHeader('Content-Type', 'application/json')
    response.end(JSON.stringify({
      method: request.method,
      url: request.url,
      headers: request.headers,
      bytes: body.length,
      body: body.toString('utf8'),
    }))
  })
  const frontend = createServer((_request, response) => {
    response.setHeader('X-Upstream', 'frontend')
    response.end('frontend fixture')
  })
  t.after(async () => {
    for (const server of [backend, frontend]) {
      server.closeAllConnections()
      await new Promise((resolve) => server.close(resolve))
    }
  })
  await Promise.all([
    once(backend.listen(8080, '0.0.0.0'), 'listening'),
    once(frontend.listen(3000, '0.0.0.0'), 'listening'),
  ])

  let ready = false
  for (let attempt = 0; attempt < 60; attempt += 1) {
    try {
      const response = await fetch(baseURL, { signal: AbortSignal.timeout(1000) })
      ready = response.ok && await response.text() === 'frontend fixture'
      if (ready) break
    } catch {
      // Compose starts both services together; allow the proxy and DNS to settle.
    }
    await delay(250)
  }
  assert.ok(ready, 'gateway did not become ready')

  await t.test('routes pages and API paths without changing query strings', async () => {
    for (const path of ['/', '/workspace/projects', '/api/v10/example']) {
      const response = await fetch(`${baseURL}${path}`)
      assert.equal(response.headers.get('x-upstream'), 'frontend')
      assert.equal(await response.text(), 'frontend fixture')
    }
    for (const path of ['/api/v1', '/api/v1/records?code=a%2Fb&code=c']) {
      const response = await fetch(`${baseURL}${path}`)
      assert.equal(response.headers.get('x-upstream'), 'backend')
      assert.equal(response.headers.get('x-content-type-options'), 'nosniff')
      assert.equal(response.headers.get('server'), null)
      assert.equal((await response.json()).url, path)
    }
  })

  await t.test('preserves authentication and rejects forged proxy headers', async () => {
    const response = await fetch(`${baseURL}/api/v1/headers`, {
      headers: {
        Authorization: 'Bearer gateway-test-fixture',
        'X-Organization-ID': 'test-organization',
        'X-Request-ID': 'gateway-test-request',
        'X-Forwarded-For': '198.51.100.99',
        'X-Real-IP': '198.51.100.99',
        Forwarded: 'for=198.51.100.99;proto=https',
      },
    })
    const { headers } = await response.json()
    assert.equal(headers.authorization, 'Bearer gateway-test-fixture')
    assert.equal(headers['x-organization-id'], 'test-organization')
    assert.equal(headers['x-request-id'], 'gateway-test-request')
    assert.ok(headers['x-forwarded-for'])
    assert.ok(!headers['x-forwarded-for'].includes('198.51.100.99'))
    assert.equal(headers['x-real-ip'], headers['x-forwarded-for'])
    assert.equal(headers['x-forwarded-proto'], 'http')
    assert.equal(headers.forwarded, undefined)
  })

  await t.test('preserves uploads, downloads, and upstream errors', async () => {
    const content = 'upload fixture\n'.repeat(8192)
    const form = new FormData()
    form.append('file', new Blob([content], { type: 'text/plain' }), 'fixture.txt')
    const uploaded = await fetch(`${baseURL}/api/v1/upload`, { method: 'POST', body: form })
    const echo = await uploaded.json()
    assert.equal(echo.method, 'POST')
    assert.match(echo.headers['content-type'], /^multipart\/form-data; boundary=/)
    assert.ok(echo.body.includes(content))

    const downloaded = await fetch(`${baseURL}/api/v1/download`)
    assert.equal(downloaded.headers.get('content-disposition'), 'attachment; filename="report.csv"')
    assert.deepEqual(Buffer.from(await downloaded.arrayBuffer()), download)

    const unavailable = await fetch(`${baseURL}/api/v1/unavailable`)
    assert.equal(unavailable.status, 503)
    assert.equal(unavailable.headers.get('retry-after'), '10')
    assert.deepEqual(await unavailable.json(), { error: 'upstream unavailable' })
  })

  await t.test('delivers the first SSE event before the stream finishes', async () => {
    const response = await fetch(`${baseURL}/api/v1/stream`, { signal: AbortSignal.timeout(2000) })
    assert.equal(response.headers.get('content-type'), 'text/event-stream')
    const reader = response.body.getReader()
    try {
      const first = await reader.read()
      assert.equal(new TextDecoder().decode(first.value), 'data: first\n\n')
    } finally {
      await reader.cancel()
    }
  })

  await t.test('cancels the upstream stream when the client disconnects', async () => {
    const controller = new AbortController()
    const response = await fetch(`${baseURL}/api/v1/stream/cancel`, { signal: controller.signal })
    const reader = response.body.getReader()
    await reader.read()
    controller.abort()
    await reader.cancel().catch(() => {})
    for (let attempt = 0; attempt < 20 && !canceledStream; attempt += 1) await delay(50)
    assert.ok(canceledStream, 'disconnected client left the upstream stream running')
  })
})
