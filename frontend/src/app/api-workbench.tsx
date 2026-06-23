'use client'

import { useEffect, useMemo, useState } from 'react'
import { listPlatformRuntimeOperations, listRuntimeOperations } from '@/lib/api'
import { useI18n } from '@/lib/i18n'
import { ApiOperation, apiOperations } from '@/lib/operations'
import { MethodBadge, OperationRunner } from './operation-runner'

interface ApiWorkbenchProps {
  token: string
  domain?: string
  showDomainMenu?: boolean
  apiScope?: 'tenant' | 'platform'
}

const platformOperationDomains = new Set([
  'Layer',
  'Capability',
  'Governance',
  'Evolution',
  'Verification',
  'Observability',
  'DeveloperTools',
])
const platformIdentityPaths = new Set(['/agents/register', '/agents'])
const deprecatedTenantPathPrefixes = [
  '/finance/',
  '/inventory/',
  '/procurement/',
  '/sales/',
  '/organizations',
  '/organization/',
  '/requirements',
  '/projects',
  '/workflows/',
  '/capabilities',
  '/verification/',
  '/governance/',
  '/evolution/',
  '/meta-resources',
  '/tools',
  '/tool-executions',
  '/runtime/entities/',
  '/runtime/operations',
]

function platformOperationAvailable(operation: ApiOperation): boolean {
  if (operation.domain === 'Identity') return platformIdentityPaths.has(operation.path)
  return platformOperationDomains.has(operation.domain)
}

function tenantOperationAvailable(operation: ApiOperation): boolean {
  return !deprecatedTenantPathPrefixes.some((prefix) => operation.path.startsWith(prefix))
}

function formatRuntimeDomainLabel(domain: string, translate: (key: string) => string) {
  const directLabel = translate(domain)
  if (directLabel !== domain) return directLabel
  const moduleLabelKey = `saas.module.${domain}`
  const moduleLabel = translate(moduleLabelKey)
  return moduleLabel === moduleLabelKey ? domain : moduleLabel
}

export function ApiWorkbench({ token, domain, showDomainMenu = true, apiScope = 'tenant' }: ApiWorkbenchProps) {
  const { t } = useI18n()
  const [runtimeOperations, setRuntimeOperations] = useState<ApiOperation[]>([])
  const scopedRuntimeOperations = useMemo(
    () => (apiScope === 'platform' ? runtimeOperations.filter(platformOperationAvailable) : runtimeOperations.filter(tenantOperationAvailable)),
    [apiScope, runtimeOperations],
  )
  const scopedFallbackOperations = useMemo(
    () => (apiScope === 'platform' ? apiOperations.filter(platformOperationAvailable) : apiOperations.filter(tenantOperationAvailable)),
    [apiScope],
  )
  const operationCatalog = scopedRuntimeOperations.length > 0 ? scopedRuntimeOperations : scopedFallbackOperations
  const operationDomainsForCatalog = useMemo(() => Array.from(new Set(operationCatalog.map((operation) => operation.domain))), [operationCatalog])
  const firstOperation = domain
    ? operationCatalog.find((operation) => operation.domain === domain) ?? operationCatalog[0] ?? apiOperations[0]
    : operationCatalog[0] ?? apiOperations[0]
  const [activeDomain, setActiveDomain] = useState(firstOperation.domain)
  const [selectedOperation, setSelectedOperation] = useState<ApiOperation>(firstOperation)

  useEffect(() => {
    let cancelled = false
    const loadRuntimeOperations = apiScope === 'platform' ? listPlatformRuntimeOperations : listRuntimeOperations
    loadRuntimeOperations(token)
      .then((items) => {
        if (cancelled || items.length === 0) return
        setRuntimeOperations(items)
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [apiScope, token])

  const effectiveActiveDomain = operationCatalog.some((operation) => operation.domain === activeDomain)
    ? activeDomain
    : firstOperation.domain
  const effectiveSelectedOperation = operationCatalog.some((operation) => operation.id === selectedOperation.id)
    ? selectedOperation
    : firstOperation

  const domainOperations = useMemo(
    () => operationCatalog.filter((operation) => operation.domain === effectiveActiveDomain),
    [effectiveActiveDomain, operationCatalog],
  )

  function selectDomain(domain: string) {
    const nextOperation = operationCatalog.find((operation) => operation.domain === domain) ?? firstOperation
    setActiveDomain(domain)
    setSelectedOperation(nextOperation)
  }

  function selectOperation(operation: ApiOperation) {
    setSelectedOperation(operation)
  }

  return (
    <div className={`grid gap-5 ${showDomainMenu ? 'lg:grid-cols-[220px_280px_1fr]' : 'lg:grid-cols-[280px_1fr]'}`}>
      {showDomainMenu && (
        <aside className="rounded-lg border border-slate-200 bg-white p-3 shadow-sm">
          <div className="space-y-1">
            {operationDomainsForCatalog.map((domain) => (
              <button
                key={domain}
                type="button"
                onClick={() => selectDomain(domain)}
                className={`flex h-10 w-full items-center justify-between rounded-lg px-3 text-left text-sm font-medium transition ${
                  effectiveActiveDomain === domain
                    ? 'bg-slate-950 text-white'
                    : 'text-slate-600 hover:bg-slate-100 hover:text-slate-950'
                }`}
              >
                <span>{formatRuntimeDomainLabel(domain, t)}</span>
                <span className="text-xs opacity-70">
                  {operationCatalog.filter((operation) => operation.domain === domain).length}
                </span>
              </button>
            ))}
          </div>
        </aside>
      )}

      <section className="rounded-lg border border-slate-200 bg-white p-3 shadow-sm">
        <div className="space-y-2">
          {domainOperations.map((operation) => (
            <button
              key={operation.id}
              type="button"
              onClick={() => selectOperation(operation)}
              className={`w-full rounded-lg border p-3 text-left transition ${
                effectiveSelectedOperation.id === operation.id
                  ? 'border-slate-950 bg-slate-50'
                  : 'border-slate-200 hover:border-slate-300 hover:bg-slate-50'
              }`}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="min-w-0 truncate text-sm font-semibold text-slate-950">{t(operation.title)}</span>
                <MethodBadge method={operation.method} />
              </div>
              <p className="mt-2 truncate text-xs text-slate-500">{operation.path}</p>
            </button>
          ))}
        </div>
      </section>

      <OperationRunner key={effectiveSelectedOperation.id} token={token} operation={effectiveSelectedOperation} apiScope={apiScope} />
    </div>
  )
}
