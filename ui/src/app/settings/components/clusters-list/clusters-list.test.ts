/* eslint-env jest */
declare const test: any;
declare const expect: any;
import { compareServerVersion } from './clusters-list';

const sign = (n: number) => (n < 0 ? -1 : n > 0 ? 1 : 0);

test('compares equal versions as equal', () => {
    expect(compareServerVersion('v1.28.3', 'v1.28.3')).toEqual(0);
    expect(compareServerVersion('1.28.3', 'v1.28.3')).toEqual(0);
});

test('orders by major, then minor, then patch', () => {
    expect(sign(compareServerVersion('v1.28.3', 'v2.0.0'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.28.3', 'v1.29.0'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.28.3', 'v1.28.4'))).toEqual(-1);
    expect(sign(compareServerVersion('v2.0.0', 'v1.28.3'))).toEqual(1);
});

test('compares minor/patch numerically, not lexicographically', () => {
    expect(sign(compareServerVersion('v1.9.0', 'v1.10.0'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.28.9', 'v1.28.10'))).toEqual(-1);
});

test('is case-insensitive on the v prefix', () => {
    expect(compareServerVersion('V1.28.3', 'v1.28.3')).toEqual(0);
});

test('treats empty versions as lowest', () => {
    expect(sign(compareServerVersion('', 'v1.0.0'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.0.0', ''))).toEqual(1);
    expect(compareServerVersion('', '')).toEqual(0);
});

test('treats missing trailing components as zero', () => {
    expect(compareServerVersion('v1.28', 'v1.28.0')).toEqual(0);
    expect(sign(compareServerVersion('v1.28', 'v1.28.1'))).toEqual(-1);
});

test('ignores build metadata after a plus', () => {
    expect(compareServerVersion('v1.28.3+k3s1', 'v1.28.3')).toEqual(0);
    expect(compareServerVersion('v1.28.3+k3s1', 'v1.28.3+gke.99')).toEqual(0);
});

test('ranks a pre-release suffix below the plain release', () => {
    expect(sign(compareServerVersion('v1.28.3-gke.100', 'v1.28.3'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.28.3', 'v1.28.3-rc.1'))).toEqual(1);
});

test('compares pre-release identifiers (numeric and non-numeric)', () => {
    expect(sign(compareServerVersion('v1.0.0-alpha', 'v1.0.0-beta'))).toEqual(-1);
    expect(sign(compareServerVersion('v1.0.0-alpha.1', 'v1.0.0-alpha.2'))).toEqual(-1);
    // numeric identifiers rank below alphanumeric ones
    expect(sign(compareServerVersion('v1.0.0-1', 'v1.0.0-alpha'))).toEqual(-1);
    // a larger set of fields outranks a smaller one when the prefix matches
    expect(sign(compareServerVersion('v1.0.0-alpha', 'v1.0.0-alpha.1'))).toEqual(-1);
});

test('sorts a list of versions ascending', () => {
    const versions = ['v1.10.0', 'v1.9.0', 'v2.0.1', 'v1.10.2', '', 'v1.9.10'];
    const sorted = [...versions].sort(compareServerVersion);
    expect(sorted).toEqual(['', 'v1.9.0', 'v1.9.10', 'v1.10.0', 'v1.10.2', 'v2.0.1']);
});

test('sorts pre-releases before their release in an ascending list', () => {
    const versions = ['v1.28.3', 'v1.28.3-rc.2', 'v1.28.3-rc.1', 'v1.28.2'];
    const sorted = [...versions].sort(compareServerVersion);
    expect(sorted).toEqual(['v1.28.2', 'v1.28.3-rc.1', 'v1.28.3-rc.2', 'v1.28.3']);
});
