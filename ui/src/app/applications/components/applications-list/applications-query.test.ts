import {Application} from '../../../shared/models';
import {compileApplicationQuery} from './applications-query';
import {applicationQueryLink, applicationQueryParams} from './applications-query-url';

const app = (name: string, sync: string, health: string, project = 'production'): Application =>
    ({
        metadata: {name, namespace: 'argocd', labels: {'team': 'platform', 'app.kubernetes.io/name': name}, annotations: {'example.com/owner': 'Alice & Bob'}},
        spec: {
            project,
            destination: {namespace: 'payments', name: 'local', server: 'https://kubernetes.default.svc'},
            source: {repoURL: 'https://example.com/repo.git', targetRevision: 'main'}
        },
        status: {sync: {status: sync}, health: {status: health}}
    }) as Application;
const apps = [app('healthy', 'Synced', 'Healthy'), app('drifted', 'OutOfSync', 'Healthy'), app('degraded', 'Synced', 'Degraded', 'staging'), app('both', 'OutOfSync', 'Degraded')];
const names = (query: string) => {
    const compiled = compileApplicationQuery(query);
    expect(compiled.error).toBeUndefined();
    return apps.filter(compiled.matches).map(item => item.metadata.name);
};

test.each([
    ['', ['healthy', 'drifted', 'degraded', 'both']],
    ['sync = OutOfSync OR health = Degraded', ['drifted', 'degraded', 'both']],
    ['sync = OutOfSync AND health = Degraded', ['both']],
    ['sync = OutOfSync health = Degraded', ['both']],
    ['sync = OutOfSync and health = Degraded', ['both']],
    ['sync = OutOfSync aNd health != Healthy', ['both']],
    ['sync = OutOfSync or health = Degraded project = staging', ['drifted', 'degraded', 'both']],
    ['sync = OutOfSync health = Healthy OR project = staging', ['drifted', 'degraded']],
    ['(sync = OutOfSync or health = Degraded) project = staging', ['degraded']],
    ['sync = OutOfSync (health = Degraded or project = staging)', ['both']],
    ['(sync = OutOfSync)(health = Degraded)', ['both']],
    ['sync = OutOfSync not health = Healthy', ['both']],
    ['health in (Healthy, Degraded) project not in (staging)', ['healthy', 'drifted', 'both']],
    ['autoSync = Disabled health != Degraded', ['healthy', 'drifted']],
    ['sync = OutOfSync OR health = Degraded AND project = staging', ['drifted', 'degraded', 'both']],
    ['(sync = OutOfSync OR health = Degraded) AND project = staging', ['degraded']],
    ['NOT (sync = OutOfSync OR health = Degraded)', ['healthy']],
    ['not not health = degraded', ['degraded', 'both']],
    ['health IN (Degraded, Missing)', ['degraded', 'both']],
    ['health NOT IN (Healthy, Missing)', ['degraded', 'both']],
    ['health != Healthy', ['degraded', 'both']],
    ['name ~ grad', ['degraded']],
    ['name !~ grad', ['healthy', 'drifted', 'both']],
    ['SYNC = "out of sync" or HEALTH = degraded', ['drifted', 'degraded', 'both']],
    ['label.team = platform AND annotation.example.com/owner = "Alice & Bob"', ['healthy', 'drifted', 'degraded', 'both']],
    ['label.app.kubernetes.io/name = both', ['both']],
    ['label.missing != x', ['healthy', 'drifted', 'degraded', 'both']],
    ['label.missing = ""', []],
    ['project = staging AND namespace = payments AND appNamespace = argocd', ['degraded']],
    ['cluster = local AND cluster ~ kubernetes AND repo ~ example.com AND revision = main', ['healthy', 'drifted', 'degraded', 'both']],
    ['autoSync = Disabled AND hydration = None', ['healthy', 'drifted', 'degraded', 'both']],
    ['DEGRADED', ['degraded']],
    ['name = "AND"', []],
    ['"both" AND health = Degraded', ['both']]
])('evaluates %s', (query, expected) => expect(names(query as string)).toEqual(expected));

test('compares every source and negates the whole multi-source condition', () => {
    const multi = {
        ...apps[0],
        spec: {
            ...apps[0].spec,
            source: undefined,
            sources: [
                {repoURL: 'https://first/repo', targetRevision: 'main'},
                {repoURL: 'https://second/repo', targetRevision: 'v2'}
            ]
        }
    } as Application;
    expect(compileApplicationQuery('repo ~ second AND revision = v2').matches(multi)).toBe(true);
    expect(compileApplicationQuery('repo != "https://first/repo"').matches(multi)).toBe(false);
});

test('supports quoted escaped values and treats values as data', () => {
    const special = {...apps[0], metadata: {...apps[0].metadata, annotations: {note: 'say "hello" \\ world', code: '<script>alert(1)</script>'}}};
    expect(compileApplicationQuery('annotation.note = "say \\"hello\\" \\\\ world"').matches(special)).toBe(true);
    expect(compileApplicationQuery('annotation.code = "<script>alert(1)</script>"').matches(special)).toBe(true);
});

test.each(['label', 'annotation'])('only reads own %s metadata, including keys that also exist on Object.prototype', field => {
    for (const key of ['constructor', 'toString', '__proto__']) {
        expect(compileApplicationQuery(`${field}.${key} = platform`).matches(apps[0])).toBe(false);
        expect(compileApplicationQuery(`${field}.${key} != platform`).matches(apps[0])).toBe(true);
        const metadata = field === 'label' ? 'labels' : 'annotations';
        const own = {...apps[0], metadata: {...apps[0].metadata, [metadata]: {[key]: 'platform'}}};
        expect(compileApplicationQuery(`${field}.${key} = platform`).matches(own)).toBe(true);
        expect(compileApplicationQuery(`${field}.${key} NOT IN (platform)`).matches(own)).toBe(false);
    }
});

test('preserves unknown escapes and distinguishes a literal backslash from a newline', () => {
    const special = {...apps[0], metadata: {...apps[0].metadata, annotations: {note: String.raw`\d\n`}}};
    expect(compileApplicationQuery(String.raw`annotation.note = "\d\\n"`).matches(special)).toBe(true);
    expect(compileApplicationQuery(String.raw`annotation.note = "\d\n"`).matches(special)).toBe(false);
});

test('favorite is local to the supplied user preferences and can be inside OR', () => {
    const query = compileApplicationQuery('favorite = true OR health = Degraded', {favorites: ['argocd/drifted']});
    expect(apps.filter(query.matches).map(item => item.metadata.name)).toEqual(['drifted', 'degraded', 'both']);
});

test.each([
    'health =',
    'health IN ()',
    'health IN (Healthy,)',
    '(health = Healthy',
    'health = Healthy)',
    'health = Healthy OR',
    'health = and sync = Synced',
    'sync = OutOfSync health =',
    'healt = Degraded',
    'health = "unclosed',
    'health ! Healthy',
    'favorite = yes',
    'NOT',
    'AND health = Healthy'
])('reports invalid input without broadening matches: %s', query => {
    const result = compileApplicationQuery(query);
    expect(result.error?.position).toBeGreaterThan(0);
    expect(result.error?.message).toBeTruthy();
    expect(apps.filter(result.matches)).toEqual([]);
});

test('bounds input size, nesting and token count', () => {
    for (const query of ['a'.repeat(4097), 'NOT '.repeat(34) + 'health = Healthy', '('.repeat(34) + 'health = Healthy' + ')'.repeat(34), Array(130).fill('a = b').join(' OR ')]) {
        expect(compileApplicationQuery(query).error).toBeDefined();
    }
});

test('copies the exact query including URL punctuation, quotes and Unicode', () => {
    const query = 'annotation.owner = "Алиса & Bob + % # ?" AND (health = Degraded OR sync = OutOfSync)';
    const copied = new URL(applicationQueryLink('http://localhost:4000/argo/applications?proj=old&sync=Synced&appNamespace=hidden&search=old&page=9&new=%7B%7D&view=list', query));
    expect(copied.pathname).toBe('/argo/applications');
    expect(copied.searchParams.get('q')).toBe(query);
    expect(copied.searchParams.get('view')).toBe('list');
    expect(copied.searchParams.get('page')).toBe('0');
    ['proj', 'sync', 'appNamespace', 'search', 'new'].forEach(key => expect(copied.searchParams.has(key)).toBe(false));
    expect(applicationQueryParams('').q).toBe('');
});
