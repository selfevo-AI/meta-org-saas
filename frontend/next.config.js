const { PHASE_DEVELOPMENT_SERVER } = require('next/constants')

/** @type {import('next').NextConfig} */
const nextConfig = {
  allowedDevOrigins: ['127.0.0.1', '172.16.0.2'],
  output: 'standalone',
}

module.exports = (phase) => {
  if (phase !== PHASE_DEVELOPMENT_SERVER) return nextConfig

  const upstream = new URL(process.env.API_PROXY_TARGET || 'http://127.0.0.1:8080')
  if (!['http:', 'https:'].includes(upstream.protocol) || upstream.username || upstream.password
    || upstream.pathname !== '/' || upstream.search || upstream.hash) {
    throw new Error('API_PROXY_TARGET must be an HTTP(S) origin without credentials, path, query, or fragment')
  }

  return {
    ...nextConfig,
    experimental: { proxyTimeout: 600_000, proxyClientMaxBodySize: '32mb' },
    async rewrites() {
      return [{ source: '/api/v1/:path*', destination: `${upstream.origin}/api/v1/:path*` }]
    },
  }
}
