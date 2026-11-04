'use client'

import { ArrowRight, Boxes, ClipboardCheck, FolderKanban, RefreshCw, ShieldCheck, Sparkles, Users } from 'lucide-react'
import type { FormEvent } from 'react'

import { useI18n } from '@/lib/i18n'
import { FeedbackMessage } from './workspace-ui'

export type AuthMode = 'login' | 'register'
export type LoginSurface = 'tenant' | 'platform'

export interface LoginSurfaceProps {
  mode: AuthMode
  loginSurface: LoginSurface
  name: string
  email: string
  password: string
  error: string | null
  notice: string | null
  loading: boolean
  onNameChange: (value: string) => void
  onEmailChange: (value: string) => void
  onPasswordChange: (value: string) => void
  onModeChange: (mode: AuthMode) => void
  onSurfaceChange: (surface: LoginSurface) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
}

export function LoginSurfaceView({
  mode,
  loginSurface,
  name,
  email,
  password,
  error,
  notice,
  loading,
  onNameChange,
  onEmailChange,
  onPasswordChange,
  onModeChange,
  onSurfaceChange,
  onSubmit,
}: LoginSurfaceProps) {
  const { locale, setLocale, t } = useI18n()
  return <div data-testid="login-shell" className="login-shell">
    <section className="ui-card login-form">
      <div className="sidebar-brand">
        <span className="sidebar-brand-mark"><Boxes size={23} strokeWidth={1.8} /></span>
        <div><strong>{t('app.product')}</strong><small>{t('ui.brand.tagline')}</small></div>
      </div>
      <h1>{t(mode === 'login' ? 'ui.login.formTitle' : 'auth.createAccount')}</h1>
      <p className="login-form-hint">{t('ui.login.formHint')}</p>
      <div className="login-surface-switch">
        <button type="button" data-testid="login-surface-tenant" onClick={() => onSurfaceChange('tenant')} aria-pressed={loginSurface === 'tenant'} title={t('auth.organizationLogin')}>
          <Users size={16} />{t('auth.tenantConsole')}
        </button>
        <button type="button" data-testid="login-surface-platform" onClick={() => onSurfaceChange('platform')} aria-pressed={loginSurface === 'platform'} title={t('auth.platformLogin')}>
          <ShieldCheck size={16} />{t('auth.platformAdmin')}
        </button>
      </div>
      <form className="mt-6 space-y-5" onSubmit={onSubmit}>
        {loginSurface === 'tenant' && mode === 'register' && <label className="ui-field"><span className="ui-field-label">{t('auth.name')}</span>
          <input value={name} onChange={(event) => onNameChange(event.target.value)} className="ui-input" autoComplete="name" required />
        </label>}
        <label className="ui-field"><span className="ui-field-label">{t('auth.email')}</span>
          <input data-testid="auth-email" value={email} onChange={(event) => onEmailChange(event.target.value)} className="ui-input" autoComplete="email" type="email" required />
        </label>
        <label className="ui-field"><span className="ui-field-label">{t('auth.password')}</span>
          <input data-testid="auth-password" value={password} onChange={(event) => onPasswordChange(event.target.value)} className="ui-input" autoComplete={mode === 'login' ? 'current-password' : 'new-password'} type="password" required />
        </label>
        {error && <FeedbackMessage error>{error}</FeedbackMessage>}
        {notice && <FeedbackMessage>{notice}</FeedbackMessage>}
        <button type="submit" data-testid="auth-submit" disabled={loading} className="ui-button ui-button-primary w-full">
          {loading ? <RefreshCw size={16} className="animate-spin" /> : <ArrowRight size={16} />}
          {t(loading ? 'auth.processing' : mode === 'login' ? 'auth.signIn' : 'auth.createAccount')}
        </button>
      </form>
      {loginSurface === 'tenant' && <button type="button" className="ui-button ui-button-link mt-3 w-full" onClick={() => onModeChange(mode === 'login' ? 'register' : 'login')}>{t(mode === 'login' ? 'auth.register' : 'auth.login')}</button>}
      <div className="login-language"><div className="shell-language">
        <button type="button" onClick={() => setLocale('zh')} aria-pressed={locale === 'zh'}>中文</button>
        <button type="button" onClick={() => setLocale('en')} aria-pressed={locale === 'en'}>EN</button>
      </div></div>
    </section>
    <section className="login-intro">
      <span className="login-intro-label"><Boxes size={17} />{t('ui.brand.tagline')}</span>
      <h2>{t('ui.login.welcome')}</h2>
      <p>{t('ui.login.subtitle')}</p>
      <div className="login-features">
        {[
          { key: 'work', icon: FolderKanban }, { key: 'review', icon: ClipboardCheck }, { key: 'ai', icon: Sparkles },
        ].map(({ key, icon: Icon }) => <div className="login-feature" key={key}>
          <span><Icon size={20} strokeWidth={1.7} /></span>
          <div><strong>{t('ui.login.feature.' + key)}</strong><p>{t('ui.login.feature.' + key + 'Hint')}</p></div>
        </div>)}
      </div>
    </section>
  </div>
}
