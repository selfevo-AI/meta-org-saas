import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { randomBytes } from 'node:crypto'
import https from 'node:https'
import { fileURLToPath } from 'node:url'

const repository = fileURLToPath(new URL('../../', import.meta.url))
const project = `meta-org-deployment-check-${process.pid}-${Date.now()}`
const password = randomBytes(24).toString('hex')
const administratorEmail = 'platform-admin@local.test'
const administratorPassword = 'MetaOrgSaasDev!2026'
const environment = {
  ...process.env,
  APP_DOMAIN: 'localhost',
  ACME_EMAIL: 'test@example.invalid',
  HTTP_PORT: '127.0.0.1:18080',
  HTTPS_PORT: '127.0.0.1:18443',
  APP_NETWORK_SUBNET: '172.30.41.0/24',
  APP_NETWORK_DYNAMIC_RANGE: '172.30.41.128/25',
  PROXY_IP_ADDRESS: '172.30.41.2',
  POSTGRES_PASSWORD: password,
  PLATFORM_DATABASE_URL: `postgres://postgres:${password}@postgres:5432/meta_org_saas?sslmode=disable`,
  TENANT_DATABASE_ADMIN_URL: `postgres://postgres:${password}@postgres:5432/postgres?sslmode=disable`,
  JWT_SECRET: randomBytes(32).toString('hex'),
  MODEL_SECRET_KEY: randomBytes(16).toString('hex'),
  SECURITY_KERNEL_SHARED_SECRET: randomBytes(32).toString('hex'),
  META_ORG_PLATFORM_ADMIN_EMAIL: administratorEmail,
  META_ORG_PLATFORM_ADMIN_PASSWORD_HASH: '$2a$10$/Dou0gOhCVFNGMitu8IUu.92HzEaG6iYWGxTTVUrSA1pkFvogvj22',
}
const images = {
  frontend: process.env.DEPLOY_TEST_FRONTEND_IMAGE,
  backend: process.env.DEPLOY_TEST_BACKEND_IMAGE,
  'security-kernel': process.env.DEPLOY_TEST_KERNEL_IMAGE,
}
const suppliedImages = Object.values(images).filter(Boolean).length
assert.ok(suppliedImages === 0 || suppliedImages === 3, 'Supply all three DEPLOY_TEST_*_IMAGE variables, or none')
const reuseImages = suppliedImages === 3
const override = JSON.stringify({
  services: reuseImages ? Object.fromEntries(Object.entries(images).map(([service, image]) => [service, { image }])) : {},
})
const baseArgs = ['compose', '-p', project, '-f', 'docker-compose.production.yml', '-f', '-']

function compose(args, quiet = false) {
  const result = spawnSync('docker', [...baseArgs, ...args], {
    cwd: repository,
    input: override,
    env: environment,
    encoding: 'utf8',
    timeout: 20 * 60_000,
    maxBuffer: 16 * 1024 * 1024,
  })
  if (!quiet && result.stdout) process.stdout.write(result.stdout)
  if (!quiet && result.stderr) process.stdout.write(result.stderr)
  if (result.status !== 0) {
    if (quiet && result.stderr) process.stdout.write(result.stderr)
    throw new Error(`docker compose ${args[0]} failed: ${result.error?.message || `exit ${result.status}`}`)
  }
  return result.stdout
}

function request(path, method = 'GET', body, token) {
  return new Promise((resolve, reject) => {
    const headers = { 'Content-Type': 'application/json', Origin: 'https://localhost' }
    if (token) headers.Authorization = `Bearer ${token}`
    const outgoing = https.request(`https://localhost:18443${path}`, {
      method,
      headers,
      // Only this loopback fixture uses Caddy's temporary local certificate.
      rejectUnauthorized: false,
      timeout: 30_000,
    }, (response) => {
      let data = ''
      response.setEncoding('utf8')
      response.on('data', chunk => { data += chunk })
      response.on('end', () => {
        try {
          const json = response.headers['content-type']?.includes('application/json') ? JSON.parse(data) : undefined
          resolve({ status: response.statusCode, headers: response.headers, json })
        } catch (error) {
          reject(error)
        }
      })
      response.on('error', reject)
    })
    outgoing.on('error', reject)
    outgoing.on('timeout', () => outgoing.destroy(new Error('HTTPS request timed out')))
    outgoing.end(body === undefined ? undefined : JSON.stringify(body))
  })
}

console.log(`Starting isolated production configuration: ${project}`)
try {
  compose(['up', '-d', reuseImages ? '--no-build' : '--build', '--wait', '--wait-timeout', '180'])
  const health = await request('/api/v1/health')
  assert.equal(health.status, 200)
  assert.equal(health.json.status, 'ok')
  assert.equal(health.json.platform_database.status, 'ok')
  assert.equal(health.json.security_kernel.status, 'ok')
  const page = await request('/')
  assert.equal(page.status, 200)
  assert.equal(page.headers['strict-transport-security'], 'max-age=31536000')

  const login = await request('/api/v1/auth/login', 'POST', { email: administratorEmail, password: administratorPassword })
  assert.equal(login.status, 200)
  assert.ok(login.json.token)
  const providers = await request('/api/v1/platform/admin/model-providers?limit=1', 'GET', undefined, login.json.token)
  assert.equal(providers.status, 200)
  const sample = await request('/api/v1/platform/admin/sample-tenants/business-closure', 'POST', {}, login.json.token)
  assert.equal(sample.status, 201)
  const tenantLogin = await request('/api/v1/auth/login', 'POST', { email: 'demo@local.com', password: 'MetaOrgSampleTenant!2026' })
  assert.equal(tenantLogin.status, 200)
  assert.ok(tenantLogin.json.token)
  let tenantReady = false
  for (let attempt = 0; attempt < 45; attempt += 1) {
    const projects = await request('/api/v1/projects?limit=1', 'GET', undefined, tenantLogin.json.token)
    if (projects.status === 200) { tenantReady = true; break }
    await new Promise(resolve => setTimeout(resolve, 1000))
  }
  assert.ok(tenantReady, 'fresh tenant did not become ready')
  const databases = compose([
    'exec', '-T', 'postgres', 'psql', '-U', 'postgres', '-d', 'postgres', '-Atc',
    "SELECT datname FROM pg_database WHERE datname ~ '^meta_org_(saas|[0-9a-f]{4})$' ORDER BY datname",
  ], true).trim().split(/\r?\n/)
  assert.ok(databases.includes('meta_org_saas'))
  assert.ok(databases.some(name => /^meta_org_[0-9a-f]{4}$/.test(name)))
  console.log('PASS: production HTTPS health, HSTS, platform login, and tenant login/provisioning on fresh databases.')
} catch (error) {
  compose(['logs', '--no-color', '--tail', '30', 'backend', 'security-kernel', 'gateway'])
  throw error
} finally {
  compose(['down', '--volumes', '--timeout', '15', ...(!reuseImages ? ['--rmi', 'local'] : [])])
  console.log('Removed the isolated test containers and their temporary volumes.')
}
