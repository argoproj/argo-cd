import {Application, HealthStatuses, HydrateOperationPhases, OperationStateTitles, SyncStatuses} from '../../../shared/models';
import {APPLICATION_QUERY_FIELDS, applicationQueryValues, ApplicationQueryToken, tokenizeApplicationQuery} from './applications-query';

export type ApplicationQueryValueIndex = Record<string, string[]>;
export interface ApplicationQuerySuggestion {
    label: string;
    description: string;
    value: string;
    cursor: number;
}

const STATUS_VALUES: ApplicationQueryValueIndex = {
    sync: Object.values(SyncStatuses),
    health: Object.values(HealthStatuses),
    operation: Object.values(OperationStateTitles),
    hydration: ['None', ...Object.values(HydrateOperationPhases)],
    autoSync: ['Enabled', 'Disabled'],
    favorite: ['true', 'false']
};

/** Index the whole authorized list, so an OR alternative is never hidden by the current query. */
export function indexApplicationQueryValues(applications: Application[]): ApplicationQueryValueIndex {
    const index = new Map<string, Set<string>>();
    for (const app of applications) {
        const fields = [
            ...APPLICATION_QUERY_FIELDS.filter(field => !(field in STATUS_VALUES)),
            ...Object.keys(app.metadata.labels || {}).map(key => `label.${key}`),
            ...Object.keys(app.metadata.annotations || {}).map(key => `annotation.${key}`)
        ];
        for (const field of fields) {
            if (!index.has(field)) index.set(field, new Set());
            for (const value of applicationQueryValues(app, field)) {
                if (value !== undefined && value !== null) index.get(field).add(value);
            }
        }
    }
    return Object.fromEntries(Array.from(index, ([field, values]) => [field, Array.from(values).sort()]));
}

type Phase = 'field' | 'operator' | 'value' | 'join' | 'not-in' | 'list-open' | 'list-value' | 'list-separator';

// Walk the completed prefix only. A value list has its own state, distinct from a boolean group.
function completionContext(tokens: ApplicationQueryToken[]) {
    let phase: Phase = 'field';
    let field = '';
    let groups = 0;
    for (const token of tokens) {
        const keyword = token.kind === 'value' ? token.text.toUpperCase() : '';
        const isValue = ['value', 'quoted'].includes(token.kind) && !['AND', 'OR', 'NOT', 'IN'].includes(keyword);
        if (phase === 'operator') {
            if (token.kind === 'operator') {
                phase = 'value';
                continue;
            }
            if (keyword === 'IN' || keyword === 'NOT') {
                phase = keyword === 'IN' ? 'list-open' : 'not-in';
                continue;
            }
            phase = 'join'; // The preceding word was a plain name search.
        }
        if (phase === 'join') {
            if (keyword === 'AND' || keyword === 'OR') {
                phase = 'field';
                continue;
            }
            if (token.kind === ')') {
                if (--groups < 0) return;
                continue;
            }
            phase = 'field'; // Implicit AND.
        }
        if (phase === 'field') {
            if (keyword === 'NOT') continue;
            if (token.kind === '(') {
                groups++;
            } else if (isValue) {
                field = token.text;
                phase = 'operator';
            } else return;
        } else if (phase === 'value' || phase === 'list-value') {
            if (!isValue) return;
            phase = phase === 'value' ? 'join' : 'list-separator';
        } else if (phase === 'not-in') {
            if (keyword !== 'IN') return;
            phase = 'list-open';
        } else if (phase === 'list-open') {
            if (token.kind !== '(') return;
            phase = 'list-value';
        } else if (phase === 'list-separator') {
            if (token.kind === ',') phase = 'list-value';
            else if (token.kind === ')') phase = 'join';
            else return;
        }
    }
    return {phase, field, groups};
}

function quoteValue(value: string) {
    const quoted = JSON.stringify(value);
    return !value || /[\s(),=!~"']/.test(value) || /^(AND|OR|NOT|IN)$/i.test(value) || quoted.slice(1, -1) !== value ? quoted : value;
}

/** Replace only the token at the caret, leaving the rest of the expression intact. */
export function completeApplicationQuery(input: string, cursor = input.length, values: ApplicationQueryValueIndex = {}): ApplicationQuerySuggestion[] {
    if (input.length > 4096) return [];
    cursor = Math.max(0, Math.min(cursor, input.length));
    try {
        const prefix = tokenizeApplicationQuery(input.slice(0, cursor), true).filter(token => token.kind !== 'end');
        const last = prefix[prefix.length - 1];
        const closedQuote = last?.kind === 'quoted' && last.closed;
        const active = last?.end === cursor && (last.kind === 'value' || (last.kind === 'quoted' && !closedQuote) || (last.kind === 'operator' && last.text === '!'));
        const start = active ? prefix.pop().position : cursor;
        const context = completionContext(prefix);
        if (!context) return [];
        const {phase, field, groups} = context;
        const fragment = active ? last.text : '';
        const tokenAtCursor = tokenizeApplicationQuery(input, true).find(token => token.position === start && ['value', 'quoted', 'operator'].includes(token.kind));
        const end = tokenAtCursor?.end ?? cursor;
        const fields = Array.from(new Set([...APPLICATION_QUERY_FIELDS, ...Object.keys(values), 'label.', 'annotation.']));
        const candidates: Array<{label: string; description: string; text?: string}> = [];
        const add = (labels: string[], description: string) => labels.forEach(label => candidates.push({label, description}));
        if (phase === 'field' || phase === 'join') {
            if (phase === 'join') {
                candidates.push({label: 'AND', description: 'Match both conditions'}, {label: 'OR', description: 'Match either condition'});
                if (groups) add([')'], 'Close group');
            }
            add(fields, phase === 'join' ? 'Field · AND implied' : 'Field');
            add(['NOT'], 'Exclude condition');
            add(['('], 'Start group');
        } else if (phase === 'operator') {
            candidates.push(
                {label: '=', description: 'Equals'},
                {label: '!=', description: 'Does not equal'},
                {label: '~', description: 'Contains'},
                {label: '!~', description: 'Does not contain'},
                {label: 'IN', description: 'Any listed value', text: 'IN ('},
                {label: 'NOT IN', description: 'Exclude listed values', text: 'NOT IN ('}
            );
        } else if (phase === 'value' || phase === 'list-value') {
            // Metadata keys are case sensitive; built-in field names are not.
            const key = APPLICATION_QUERY_FIELDS.find(name => name.toLowerCase() === field.toLowerCase()) || field;
            for (const value of Array.from(new Set([...(STATUS_VALUES[key] || []), ...(values[key] || [])]))) {
                candidates.push({label: value || '""', description: `${key} value`, text: quoteValue(value)});
            }
        } else if (phase === 'not-in') add(['IN'], 'Start value list');
        else if (phase === 'list-open') add(['('], 'Start value list');
        else if (phase === 'list-separator') {
            add([','], 'Add another value');
            add([')'], 'Finish value list');
        }
        const normalize = (text: string) => text.toLowerCase().replace(/[\s_-]/g, '');
        return candidates
            .filter(item => normalize(item.label).startsWith(normalize(fragment)))
            .slice(0, 30)
            .map(item => {
                const before = input.slice(0, start);
                const after = input.slice(end);
                const text = item.text ?? item.label;
                const spaceBefore = before && !/[\s(]$/.test(before) && ![',', ')'].includes(text) ? ' ' : '';
                const spaceAfter = after.startsWith(' ') || text.endsWith('(') || text.endsWith('.') ? '' : ' ';
                const replacement = `${spaceBefore}${text}${spaceAfter}`;
                return {...item, value: before + replacement + after, cursor: before.length + replacement.length + (after.startsWith(' ') ? 1 : 0)};
            });
    } catch {
        // Oversized token streams or malformed prefixes must not interrupt editing.
        return [];
    }
}
