import {format, parse} from './kustomize-image';
import {execFileSync} from 'child_process';
import {readFileSync} from 'fs';
import {ModuleKind, transpileModule} from 'typescript';

test('parse image version override', () => {
    const image = parse('foo/bar:v1.0.0');

    expect(image.name).toBe('foo/bar');
    expect(image.newTag).toBe('v1.0.0');
});

test('format image version override', () => {
    const formatted = format({name: 'foo/bar', newTag: 'v1.0.0'});
    expect(formatted).toBe('foo/bar:v1.0.0');
});

test('parse image name override', () => {
    const image = parse('foo/bar=foo/bar1:v1.0.0');

    expect(image.name).toBe('foo/bar');
    expect(image.newName).toBe('foo/bar1');
    expect(image.newTag).toBe('v1.0.0');
});

test('format image name override', () => {
    const formatted = format({name: 'foo/bar', newTag: 'v1.0.0', newName: 'foo/bar1'});
    expect(formatted).toBe('foo/bar=foo/bar1:v1.0.0');
});

test('parse image digest override', () => {
    const image = parse('foo/bar@sha:123');

    expect(image.name).toBe('foo/bar');
    expect(image.digest).toBe('sha:123');
});

test('format image digest override', () => {
    const formatted = format({name: 'foo/bar', digest: 'sha:123'});
    expect(formatted).toBe('foo/bar@sha:123');
});

test.each([
    ['nginx:${ARGOCD_APP_REVISION}', 'nginx', undefined, '${ARGOCD_APP_REVISION}'],
    ['nginx:$ARGOCD_APP_REVISION', 'nginx', undefined, '$ARGOCD_APP_REVISION'],
    ['registry.example.com:5000/nginx:release-${TAG}', 'registry.example.com:5000/nginx', undefined, 'release-${TAG}'],
    ['nginx=${REGISTRY}/nginx:${TAG}', 'nginx', '${REGISTRY}/nginx', '${TAG}']
])('keeps the image name separate from the variable tag in %s', (value, name, newName, newTag) => {
    const image = parse(value);
    expect(image.name).toBe(name);
    expect(image.newName).toBe(newName);
    expect(image.newTag).toBe(newTag);
    expect(format(image)).toBe(value);
    expect(format({...image, newTag: '2'})).toBe(newName ? `${name}=${newName}:2` : `${name}:2`);
});

test.each(['nginx:' + '$AA'.repeat(32) + '!', 'nginx:$' + 'a0$'.repeat(32) + '!'])('bounds parsing time for malformed repeated variables in %s', value => {
    const source = readFileSync(require.resolve('./kustomize-image'), 'utf8');
    const compiled = transpileModule(source, {compilerOptions: {module: ModuleKind.CommonJS}}).outputText;
    const script = `(function(exports) { ${compiled}\nprocess.stdout.write(JSON.stringify(exports.parse(${JSON.stringify(value)}))); })({});`;
    const output = execFileSync(process.execPath, ['-e', script], {encoding: 'utf8', timeout: 2000, killSignal: 'SIGKILL'});
    expect(JSON.parse(output)).toEqual({name: value});
});
