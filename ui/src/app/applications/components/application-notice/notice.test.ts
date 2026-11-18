import {
    MAX_DISMISSED_NOTICES,
    MAX_TOOLTIP_LENGTH,
    NoticeAnnotationContent,
    NoticeAnnotationIcon,
    NoticeAnnotationPermanent,
    NoticeAnnotationScope,
    NoticeAnnotationSeverity,
    NoticeAnnotationURL,
    addDismissal,
    dismissalKey,
    parseNotice,
    tooltipPreview
} from './notice';

describe('parseNotice', () => {
    test('returns null when annotations are missing', () => {
        expect(parseNotice(undefined)).toBeNull();
        expect(parseNotice(null)).toBeNull();
        expect(parseNotice({})).toBeNull();
    });

    test('returns null when content annotation is missing or empty', () => {
        expect(parseNotice({[NoticeAnnotationSeverity]: 'warning'})).toBeNull();
        expect(parseNotice({[NoticeAnnotationContent]: ''})).toBeNull();
        expect(parseNotice({[NoticeAnnotationContent]: '   '})).toBeNull();
    });

    test('parses content with defaults', () => {
        const notice = parseNotice({[NoticeAnnotationContent]: 'hello'});
        expect(notice).not.toBeNull();
        expect(notice!.content).toBe('hello');
        expect(notice!.severity).toBe('info');
        expect(notice!.permanent).toBe(false);
        expect(notice!.scope).toBe('both');
        expect(notice!.iconClass).toBe('fa-info-circle');
        expect(notice!.url).toBeUndefined();
    });

    test('coerces severity, falling back to info for unknown values', () => {
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationSeverity]: 'WARNING'})!.severity).toBe('warning');
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationSeverity]: 'critical'})!.severity).toBe('critical');
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationSeverity]: 'fatal'})!.severity).toBe('info');
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationSeverity]: ''})!.severity).toBe('info');
    });

    test('severity-derived default icon', () => {
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationSeverity]: 'warning'})!.iconClass).toBe('fa-exclamation-triangle');
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationSeverity]: 'critical'})!.iconClass).toBe('fa-exclamation-circle');
    });

    test('icon allowlist accepts known classes', () => {
        const notice = parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationIcon]: 'fa-bell'});
        expect(notice!.iconClass).toBe('fa-bell');
    });

    test('icon allowlist rejects unknown classes and falls back to severity default', () => {
        const notice = parseNotice({
            [NoticeAnnotationContent]: 'x',
            [NoticeAnnotationSeverity]: 'critical',
            [NoticeAnnotationIcon]: 'applications-tiles__selected'
        });
        expect(notice!.iconClass).toBe('fa-exclamation-circle');
    });

    test('icon allowlist rejects free-form CSS-like fragments', () => {
        const notice = parseNotice({
            [NoticeAnnotationContent]: 'x',
            [NoticeAnnotationIcon]: 'fa-info-circle; position: fixed'
        });
        expect(notice!.iconClass).toBe('fa-info-circle');
    });

    test('URL accepts http and https, returning canonical form', () => {
        // new URL().href appends a trailing slash to bare-host URLs.
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationURL]: 'https://example.com'})!.url).toBe('https://example.com/');
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationURL]: 'http://example.com/path?q=1'})!.url).toBe('http://example.com/path?q=1');
    });

    test('URL canonicalization strips embedded tab and newline characters', () => {
        const messy = 'https://example.com/\tpath\n';
        const url = parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationURL]: messy})!.url!;
        expect(url).not.toContain('\t');
        expect(url).not.toContain('\n');
        expect(url.startsWith('https://example.com/')).toBe(true);
    });

    test('URL rejects javascript:, data:, and other schemes', () => {
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationURL]: 'javascript:alert(1)'})!.url).toBeUndefined();
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationURL]: 'data:text/html,<h1>x</h1>'})!.url).toBeUndefined();
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationURL]: 'file:///etc/passwd'})!.url).toBeUndefined();
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationURL]: 'not a url'})!.url).toBeUndefined();
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationURL]: ''})!.url).toBeUndefined();
    });

    test('permanent parses "true" only', () => {
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationPermanent]: 'true'})!.permanent).toBe(true);
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationPermanent]: 'TRUE'})!.permanent).toBe(true);
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationPermanent]: 'false'})!.permanent).toBe(false);
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationPermanent]: 'yes'})!.permanent).toBe(false);
    });

    test('scope coerces to known values, defaults to both', () => {
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationScope]: 'banner'})!.scope).toBe('banner');
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationScope]: 'icon'})!.scope).toBe('icon');
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationScope]: 'BOTH'})!.scope).toBe('both');
        expect(parseNotice({[NoticeAnnotationContent]: 'x', [NoticeAnnotationScope]: 'unknown'})!.scope).toBe('both');
    });

    test('content is truncated at 500 chars with ellipsis', () => {
        const long = 'a'.repeat(600);
        const notice = parseNotice({[NoticeAnnotationContent]: long});
        expect(notice!.content).toHaveLength(500);
        expect(notice!.content.endsWith('…')).toBe(true);
    });

    test('content under the limit is preserved verbatim, including HTML-looking text', () => {
        const raw = '<script>alert(1)</script> deprecated';
        const notice = parseNotice({[NoticeAnnotationContent]: raw});
        expect(notice!.content).toBe(raw);
    });

    test('strips Unicode bidi-override characters (LRE/RLE/PDF/LRO/RLO and LRI/RLI/FSI/PDI)', () => {
        const codepoints = [0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069];
        const raw = 'a' + codepoints.map(c => String.fromCodePoint(c)).join('') + 'b';
        const notice = parseNotice({[NoticeAnnotationContent]: raw});
        expect(notice!.content).toBe('ab');
    });
});

describe('tooltipPreview', () => {
    test('passes through short content unchanged', () => {
        expect(tooltipPreview('hello')).toBe('hello');
    });

    test('truncates content longer than MAX_TOOLTIP_LENGTH with ellipsis', () => {
        const long = 'a'.repeat(MAX_TOOLTIP_LENGTH + 50);
        const preview = tooltipPreview(long);
        expect(preview).toHaveLength(MAX_TOOLTIP_LENGTH);
        expect(preview.endsWith('…')).toBe(true);
    });
});

describe('addDismissal', () => {
    test('adds a key to an empty map', () => {
        expect(addDismissal(undefined, 'k1')).toEqual({k1: true});
        expect(addDismissal({}, 'k1')).toEqual({k1: true});
    });

    test('preserves existing entries', () => {
        const result = addDismissal({a: true, b: true}, 'c');
        expect(result).toEqual({a: true, b: true, c: true});
    });

    test('caps at MAX_DISMISSED_NOTICES, dropping oldest first', () => {
        const existing: {[k: string]: boolean} = {};
        for (let i = 0; i < MAX_DISMISSED_NOTICES; i++) {
            existing[`key-${i}`] = true;
        }
        const result = addDismissal(existing, 'key-new');
        const keys = Object.keys(result);
        expect(keys).toHaveLength(MAX_DISMISSED_NOTICES);
        expect(keys[0]).toBe('key-1');
        expect(keys[keys.length - 1]).toBe('key-new');
    });

    test('does not grow when re-adding an existing key', () => {
        const existing = {a: true, b: true, c: true};
        const result = addDismissal(existing, 'b');
        expect(Object.keys(result)).toHaveLength(3);
        expect(result).toEqual({a: true, b: true, c: true});
    });
});

describe('dismissalKey', () => {
    test('changes when content changes (re-shows banner after edit)', () => {
        const before = dismissalKey('argocd', 'guestbook', 'old');
        const after = dismissalKey('argocd', 'guestbook', 'new');
        expect(before).not.toBe(after);
    });

    test('is stable across calls with the same input', () => {
        expect(dismissalKey('argocd', 'guestbook', 'msg')).toBe(dismissalKey('argocd', 'guestbook', 'msg'));
    });

    test('separates different applications with the same content', () => {
        const a = dismissalKey('argocd', 'app-a', 'msg');
        const b = dismissalKey('argocd', 'app-b', 'msg');
        expect(a).not.toBe(b);
    });
});
