import {format, parse} from './kustomize-image';

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
