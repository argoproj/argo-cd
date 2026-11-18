import {hashCode, isValidManagedByURL} from '../../../shared/utils';

export const NoticeAnnotationPrefix = 'notice.argocd.argoproj.io/';

export const NoticeAnnotationContent = `${NoticeAnnotationPrefix}content`;
export const NoticeAnnotationSeverity = `${NoticeAnnotationPrefix}severity`;
export const NoticeAnnotationURL = `${NoticeAnnotationPrefix}url`;
export const NoticeAnnotationPermanent = `${NoticeAnnotationPrefix}permanent`;
export const NoticeAnnotationIcon = `${NoticeAnnotationPrefix}icon`;
export const NoticeAnnotationScope = `${NoticeAnnotationPrefix}scope`;

export type NoticeSeverity = 'info' | 'warning' | 'critical';
export type NoticeScope = 'banner' | 'icon' | 'both';

export interface ParsedNotice {
    content: string;
    severity: NoticeSeverity;
    url?: string;
    permanent: boolean;
    iconClass: string;
    scope: NoticeScope;
}

const MAX_CONTENT_LENGTH = 500;
export const MAX_TOOLTIP_LENGTH = 200;

const SEVERITY_DEFAULT: NoticeSeverity = 'info';
const SCOPE_DEFAULT: NoticeScope = 'both';

const VALID_SEVERITIES: ReadonlySet<NoticeSeverity> = new Set<NoticeSeverity>(['info', 'warning', 'critical']);
const VALID_SCOPES: ReadonlySet<NoticeScope> = new Set<NoticeScope>(['banner', 'icon', 'both']);

const SEVERITY_DEFAULT_ICON: Record<NoticeSeverity, string> = {
    info: 'fa-info-circle',
    warning: 'fa-exclamation-triangle',
    critical: 'fa-exclamation-circle'
};

// Allowlist of FontAwesome class fragments. Only these may be set via the icon
// annotation; anything else falls back to the severity default. This prevents a
// malicious or careless annotation value from injecting arbitrary class names
// (e.g. layout-affecting utility classes) into the rendered <i>.
const ICON_ALLOWLIST: ReadonlySet<string> = new Set([
    'fa-info-circle',
    'fa-exclamation-triangle',
    'fa-exclamation-circle',
    'fa-bell',
    'fa-wrench',
    'fa-clock',
    'fa-bullhorn',
    'fa-life-ring',
    'fa-shield-alt'
]);

function coerceSeverity(raw: string | undefined): NoticeSeverity {
    if (!raw) {
        return SEVERITY_DEFAULT;
    }
    const value = raw.trim().toLowerCase() as NoticeSeverity;
    return VALID_SEVERITIES.has(value) ? value : SEVERITY_DEFAULT;
}

function coerceScope(raw: string | undefined): NoticeScope {
    if (!raw) {
        return SCOPE_DEFAULT;
    }
    const value = raw.trim().toLowerCase() as NoticeScope;
    return VALID_SCOPES.has(value) ? value : SCOPE_DEFAULT;
}

function coerceBool(raw: string | undefined): boolean {
    if (!raw) {
        return false;
    }
    return raw.trim().toLowerCase() === 'true';
}

// sanitizeURL returns the canonical http(s) form of a URL or undefined if it
// is missing, malformed, or uses any other scheme. Reuses the http(s) predicate
// from isValidManagedByURL; returning parsed.href strips embedded whitespace
// the URL parser drops.
function sanitizeURL(raw: string | undefined): string | undefined {
    if (!raw) {
        return undefined;
    }
    const trimmed = raw.trim();
    if (!trimmed || !isValidManagedByURL(trimmed)) {
        return undefined;
    }
    return new URL(trimmed).href;
}

// Strip Unicode bidi overrides — they can flip rendered text away from its
// source (e.g. presenting an attacker-controlled domain as a trusted one).
const BIDI_OVERRIDE_RE = /[‪-‮⁦-⁩]/g;
function stripBidiOverrides(content: string): string {
    return content.replace(BIDI_OVERRIDE_RE, '');
}

function pickIcon(raw: string | undefined, severity: NoticeSeverity): string {
    if (!raw) {
        return SEVERITY_DEFAULT_ICON[severity];
    }
    const value = raw.trim();
    return ICON_ALLOWLIST.has(value) ? value : SEVERITY_DEFAULT_ICON[severity];
}

function truncate(content: string, max: number = MAX_CONTENT_LENGTH): string {
    if (content.length <= max) {
        return content;
    }
    return content.slice(0, max - 1) + '…';
}

export function tooltipPreview(content: string): string {
    return truncate(content, MAX_TOOLTIP_LENGTH);
}

export function parseNotice(annotations: {[key: string]: string} | undefined | null): ParsedNotice | null {
    if (!annotations) {
        return null;
    }
    const rawContent = annotations[NoticeAnnotationContent];
    if (!rawContent || rawContent.trim() === '') {
        return null;
    }
    const severity = coerceSeverity(annotations[NoticeAnnotationSeverity]);
    return {
        content: truncate(stripBidiOverrides(rawContent)),
        severity,
        url: sanitizeURL(annotations[NoticeAnnotationURL]),
        permanent: coerceBool(annotations[NoticeAnnotationPermanent]),
        iconClass: pickIcon(annotations[NoticeAnnotationIcon], severity),
        scope: coerceScope(annotations[NoticeAnnotationScope])
    };
}

// dismissalKey is content-addressed so updating the notice content re-shows
// the banner even if the user previously dismissed it.
export function dismissalKey(appNamespace: string, appName: string, content: string): string {
    return `${appNamespace || ''}/${appName}:${hashCode(content).toString(36)}`;
}

// Cap the dismissedNotices map so it doesn't grow unbounded in localStorage
// over the lifetime of an account that dismisses many notices.
export const MAX_DISMISSED_NOTICES = 200;

// Insertion order is preserved by JS engines and survives JSON round-trip,
// so trimming the front drops oldest entries first.
export function addDismissal(existing: {[key: string]: boolean} | undefined, key: string): {[key: string]: boolean} {
    const next: {[k: string]: boolean} = {...(existing || {}), [key]: true};
    const keys = Object.keys(next);
    if (keys.length <= MAX_DISMISSED_NOTICES) {
        return next;
    }
    return keys.slice(-MAX_DISMISSED_NOTICES).reduce<{[k: string]: boolean}>((acc, k) => {
        acc[k] = next[k];
        return acc;
    }, {});
}

export function shouldShowBanner(notice: ParsedNotice): boolean {
    return notice.scope !== 'icon';
}

export function shouldShowIcon(notice: ParsedNotice): boolean {
    return notice.scope !== 'banner';
}
