import {Application} from '../../../shared/models';
import {compileApplicationQuery} from './applications-query';
import {completeApplicationQuery, indexApplicationQueryValues} from './applications-query-completion';

const labels = (query: string) => completeApplicationQuery(query).map(item => item.label);

test.each([
    ['sy', ['sync']],
    ['AUTO', ['autoSync']],
    ['(sync = Synced or hea', ['health']],
    ['sync ', ['=', '!=', '~', '!~', 'IN', 'NOT IN']],
    ['sync !', ['!=', '!~']],
    ['sync = out', ['OutOfSync']],
    ['health != de', ['Degraded']],
    ['autoSync = ', ['Enabled', 'Disabled']],
    ['favorite = f', ['false']],
    ['sync = Synced a', ['AND', 'appNamespace', 'autoSync', 'annotation.']],
    ['sync = Synced o', ['OR', 'operation']],
    ['health not ', ['IN']],
    ['health in ', ['(']],
    ['health in (Degraded, mi', ['Missing']],
    ['health in (Degraded ', [',', ')']],
    ['health = "deg', ['Degraded']],
    ['health = Degraded auto', ['autoSync']]
])('suggests the next part of %s', (query, expected) => expect(labels(query as string)).toEqual(expected));

test('offers connectors and implicit AND fields after a complete condition or group', () => {
    for (const input of ['health = Degraded ', '(health = Degraded) ', 'health in (Degraded, Missing) ', 'operation = "Sync failed"']) {
        expect(labels(input)).toEqual(expect.arrayContaining(['AND', 'OR', 'NOT', 'sync', 'autoSync', '(']));
    }
    expect(labels('(health = Degraded ')).toContain(')');
    expect(labels('health = Degraded ')).not.toContain(')');
});

test('replaces only the current token, including when editing inside a quoted value', () => {
    const input = 'sync = OutOfSync OR health = Healthy';
    const result = completeApplicationQuery(input, input.indexOf('Out') + 3).find(item => item.label === 'OutOfSync');
    expect(result.value).toBe(input);
    const quoted = 'operation = "Sync faled" AND health = Degraded';
    const option = completeApplicationQuery(quoted, quoted.indexOf('faled') + 1).find(item => item.label === 'Sync failed');
    expect(option.value).toBe('operation = "Sync failed" AND health = Degraded');
    expect(compileApplicationQuery(option.value).error).toBeUndefined();
});

test('completes lists and quotes values containing syntax or escapes', () => {
    const option = completeApplicationQuery('project = "A', undefined, {project: ['Alice "A" & Bob', 'AND']})[0];
    expect(option.value).toBe('project = "Alice \\"A\\" & Bob" ');
    expect(compileApplicationQuery(option.value).error).toBeUndefined();
    expect(completeApplicationQuery('project = AN', undefined, {project: ['AND']})[0].value).toBe('project = "AND" ');
    expect(completeApplicationQuery('health i')[0].value).toBe('health IN (');
});

test('indexes all dynamic fields and metadata without changing key case', () => {
    const app = {
        metadata: {name: 'payments', namespace: 'argocd', labels: {Team: 'Platform'}, annotations: {owner: 'Alice'}},
        spec: {project: 'production', destination: {namespace: 'payments', name: 'local'}, sources: [{repoURL: 'first'}, {repoURL: 'second'}]}
    } as Application;
    const values = indexApplicationQueryValues([app, app]);
    expect(values.project).toEqual(['production']);
    expect(values.repo).toEqual(['first', 'second']);
    expect(completeApplicationQuery('label.T', undefined, values)[0].label).toBe('label.Team');
    expect(completeApplicationQuery('label.Team = ', undefined, values)[0].value).toBe('label.Team = Platform ');
    expect(completeApplicationQuery('label.team = ', undefined, values)).toEqual([]);
    expect(completeApplicationQuery('PROJECT = pr', undefined, values)[0].label).toBe('production');
});

test.each(['first\nsecond', 'first\r\nsecond', 'first\tsecond', 'quote " and backslash \\', '\b\f\u0000\u001f', String.raw`literal\n`])(
    'round-trips an annotation suggestion containing %j through a single-line input',
    note => {
        const app = {metadata: {name: 'sample', annotations: {note}}, spec: {destination: {}, source: {}}} as Application;
        const suggestion = completeApplicationQuery('annotation.note = ', undefined, indexApplicationQueryValues([app]))[0];
        const input = document.createElement('input');
        input.type = 'text';
        input.value = suggestion.value;
        expect(input.value).toBe(suggestion.value);
        const query = compileApplicationQuery(input.value);
        expect(query.error).toBeUndefined();
        expect(query.matches(app)).toBe(true);
    }
);

test('limits suggestions without breaking editing of oversized or malformed queries', () => {
    expect(completeApplicationQuery('a'.repeat(4097))).toEqual([]);
    expect(completeApplicationQuery('health = Degraded AND OR ')).toEqual([]);
    expect(completeApplicationQuery('project = ', undefined, {project: Array.from({length: 100}, (_, i) => String(i))})).toHaveLength(30);
});
