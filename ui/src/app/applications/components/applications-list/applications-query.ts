import {Application} from '../../../shared/models';
import {getAppAllSources, getOperationStateTitle, isFavorite} from '../utils';
import {getAutoSyncStatus, getHydrationStatus} from './applications-list-stats';

export interface ApplicationQueryOptions {
    favorites?: string[];
}

export interface ApplicationQuery {
    matches: (app: Application) => boolean;
    error?: {message: string; position: number};
}

type Matcher = ApplicationQuery['matches'];
export type ApplicationQueryToken = {kind: 'value' | 'quoted' | 'operator' | '(' | ')' | ',' | 'end'; text: string; position: number; end: number; closed?: boolean};
const QUOTED_ESCAPES: Record<string, string> = {n: '\n', r: '\r', t: '\t', b: '\b', f: '\f'};

class QueryError extends Error {
    constructor(
        message: string,
        public position: number
    ) {
        super(message);
    }
}

export const APPLICATION_QUERY_FIELDS = [
    'name',
    'namespace',
    'appNamespace',
    'project',
    'cluster',
    'repo',
    'revision',
    'sync',
    'health',
    'operation',
    'hydration',
    'autoSync',
    'favorite'
];

// Completion uses the same lexer, allowing an unfinished quote or operator while typing.
export function tokenizeApplicationQuery(input: string, incomplete = false): ApplicationQueryToken[] {
    const tokens: ApplicationQueryToken[] = [];
    let offset = 0;
    while (offset < input.length) {
        if (/\s/.test(input[offset])) {
            offset++;
            continue;
        }
        const position = offset;
        const char = input[offset++];
        if (char === '"' || char === "'") {
            let text = '';
            let closed = false;
            while (offset < input.length) {
                const next = input[offset++];
                if (next === char) {
                    closed = true;
                    break;
                }
                if (next === '\\') {
                    if (offset === input.length) break;
                    const escaped = input[offset++];
                    if (escaped === char || escaped === '\\') {
                        text += escaped;
                    } else if (escaped === 'u' && /^[\da-f]{4}$/i.test(input.slice(offset, offset + 4))) {
                        text += String.fromCharCode(parseInt(input.slice(offset, offset + 4), 16));
                        offset += 4;
                    } else {
                        // JSON escapes keep suggested metadata values usable in a single-line input.
                        // Unknown escapes retain their literal backslash.
                        text += QUOTED_ESCAPES[escaped] ?? `\\${escaped}`;
                    }
                } else {
                    text += next;
                }
            }
            if (!closed && !incomplete) throw new QueryError('Close the quoted value.', position);
            tokens.push({kind: 'quoted', text, position, end: offset, closed});
        } else if (char === '(' || char === ')' || char === ',') {
            tokens.push({kind: char, text: char, position, end: offset});
        } else if ('=!~'.includes(char)) {
            let text = char;
            if (char === '!' && '=~'.includes(input[offset] || ' ')) text += input[offset++];
            if (!incomplete && !['=', '!=', '~', '!~'].includes(text)) throw new QueryError('Use =, !=, ~ or !~.', position);
            tokens.push({kind: 'operator', text, position, end: offset});
        } else {
            let text = char;
            while (offset < input.length && !/[\s(),=!~"']/.test(input[offset])) text += input[offset++];
            tokens.push({kind: 'value', text, position, end: offset});
        }
        if (tokens.length > 512) throw new QueryError('Use at most 512 query tokens.', position);
    }
    return [...tokens, {kind: 'end', text: '', position: input.length, end: input.length}];
}

export function applicationQueryValues(app: Application, field: string, options: ApplicationQueryOptions = {}): string[] {
    const lower = field.toLowerCase();
    if (lower.startsWith('label.') || lower.startsWith('annotation.')) {
        const metadata = lower.startsWith('label.') ? app.metadata.labels : app.metadata.annotations;
        const key = field.slice(field.indexOf('.') + 1);
        return metadata && Object.prototype.hasOwnProperty.call(metadata, key) ? [metadata[key]] : [];
    }
    switch (lower) {
        case 'name':
            return [app.metadata.name];
        case 'namespace':
            return [app.spec.destination.namespace];
        case 'appnamespace':
            return [app.metadata.namespace];
        case 'project':
            return [app.spec.project];
        case 'cluster':
            return [app.spec.destination.name, app.spec.destination.server];
        case 'repo':
            return getAppAllSources(app).map(source => source.repoURL);
        case 'revision':
            return getAppAllSources(app).map(source => source.targetRevision);
        case 'sync':
            return [app.status.sync?.status];
        case 'health':
            return [app.status.health?.status];
        case 'operation':
            return [getOperationStateTitle(app)];
        case 'hydration':
            return [getHydrationStatus(app)];
        case 'autosync':
            return [getAutoSyncStatus(app.spec.syncPolicy)];
        case 'favorite':
            return [String(isFavorite(options.favorites || [], app))];
    }
    return [];
}

class Parser {
    private index = 0;
    private depth = 0;
    constructor(
        private tokens: ApplicationQueryToken[],
        private options: ApplicationQueryOptions
    ) {}

    private get current() {
        return this.tokens[this.index];
    }
    private keyword(word: string) {
        return this.current.kind === 'value' && this.current.text.toUpperCase() === word;
    }
    private takeKeyword(word: string) {
        if (!this.keyword(word)) return false;
        this.index++;
        return true;
    }
    private require(kind: ApplicationQueryToken['kind'], message: string) {
        if (this.current.kind !== kind) throw new QueryError(message, this.current.position);
        return this.tokens[this.index++];
    }
    private value() {
        if (this.current.kind !== 'value' && this.current.kind !== 'quoted') throw new QueryError('Enter a value. Quote values containing spaces.', this.current.position);
        if (['AND', 'OR', 'NOT', 'IN'].some(word => this.keyword(word))) throw new QueryError('Quote reserved keywords used as values.', this.current.position);
        return this.tokens[this.index++].text;
    }

    parse(): Matcher {
        const match = this.or();
        if (this.current.kind !== 'end') throw new QueryError('Expected AND or OR between conditions.', this.current.position);
        return match;
    }
    private or(): Matcher {
        let left = this.and();
        while (this.takeKeyword('OR')) {
            const previous = left;
            const right = this.and();
            left = app => previous(app) || right(app);
        }
        return left;
    }
    private and(): Matcher {
        let left = this.term();
        while (true) {
            const explicit = this.takeKeyword('AND');
            // Adjacent conditions or groups have the same precedence as an explicit AND.
            const implicit = !this.keyword('OR') && ['value', 'quoted', '('].includes(this.current.kind);
            if (!explicit && !implicit) break;
            const previous = left;
            const right = this.term();
            left = app => previous(app) && right(app);
        }
        return left;
    }
    private term(): Matcher {
        if (++this.depth > 32) throw new QueryError('Use at most 32 nested groups or NOT operators.', this.current.position);
        let match: Matcher;
        if (this.takeKeyword('NOT')) {
            const child = this.term();
            match = app => !child(app);
        } else if (this.current.kind === '(') {
            this.index++;
            match = this.or();
            this.require(')', 'Close the group with ).');
        } else {
            match = this.condition();
        }
        this.depth--;
        return match;
    }
    private condition(): Matcher {
        const token = this.current;
        if (this.keyword('AND') || this.keyword('OR')) throw new QueryError('Enter a condition before this operator.', token.position);
        const field = this.value();
        let operator: string;
        if (this.current.kind === 'operator') {
            operator = this.tokens[this.index++].text;
        } else if (this.takeKeyword('IN')) {
            operator = 'IN';
        } else if (this.keyword('NOT') && this.tokens[this.index + 1]?.text.toUpperCase() === 'IN') {
            this.index += 2;
            operator = 'NOT IN';
        } else {
            const search = field.toLowerCase();
            return app => [app.metadata.name, app.metadata.namespace].some(value => value?.toLowerCase().includes(search));
        }
        const key = field.toLowerCase();
        if (!APPLICATION_QUERY_FIELDS.some(name => name.toLowerCase() === key) && !/^(label|annotation)\..+/.test(key)) {
            throw new QueryError(`Unknown field "${field}". Choose a field from the suggestions.`, token.position);
        }
        const values: string[] = [];
        if (operator.endsWith('IN')) {
            this.require('(', 'Start the value list with (.');
            values.push(this.value());
            while (this.current.kind === ',') {
                this.index++;
                values.push(this.value());
            }
            this.require(')', 'Separate values with commas and close the list with ).');
        } else {
            values.push(this.value());
        }
        if (key === 'favorite' && values.some(value => !['true', 'false'].includes(value.toLowerCase()))) {
            throw new QueryError('Use true or false for favorite.', token.position);
        }
        const normalize = (value: string) => {
            const lower = value.toLowerCase();
            return ['sync', 'health', 'operation', 'hydration', 'autosync'].includes(key) ? lower.replace(/[\s_-]/g, '') : lower;
        };
        const expected = values.map(normalize);
        return app => {
            const matched = applicationQueryValues(app, field, this.options)
                .filter(value => value !== undefined && value !== null)
                .some(value => {
                    const normalized = normalize(value);
                    return expected.some(target => (operator.includes('~') ? normalized.includes(target) : normalized === target));
                });
            return ['!=', '!~', 'NOT IN'].includes(operator) ? !matched : matched;
        };
    }
}

export function compileApplicationQuery(input: string, options: ApplicationQueryOptions = {}): ApplicationQuery {
    try {
        if (input.length > 4096) throw new QueryError('Use at most 4096 characters.', 4096);
        const query = input.trim();
        if (!query) return {matches: () => true};
        // A plain name or phrase remains a useful shorthand, without interpreting it as code or regex.
        if (!/[=~!(),"']|\b(?:AND|OR|NOT|IN)\b/i.test(query)) {
            const search = query.toLowerCase();
            return {matches: app => [app.metadata.name, app.metadata.namespace].some(value => value?.toLowerCase().includes(search))};
        }
        return {matches: new Parser(tokenizeApplicationQuery(input), options).parse()};
    } catch (error) {
        if (!(error instanceof QueryError)) throw error;
        return {matches: () => false, error: {message: error.message, position: error.position + 1}};
    }
}
