import * as LabelSelector from './label-selector';

test('exists', () => {
    expect(LabelSelector.match('test', {test: 'hello'})).toBeTruthy();
    expect(LabelSelector.match('test1', {test: 'hello'})).toBeFalsy();
    expect(LabelSelector.match('app.kubernetes.io/instance', {'app.kubernetes.io/instance': 'hello'})).toBeTruthy();
});

test('not exists', () => {
    expect(LabelSelector.match('!test', {test: 'hello'})).toBeFalsy();
    expect(LabelSelector.match('!test1', {test: 'hello'})).toBeTruthy();
});

test('in', () => {
    expect(LabelSelector.match('test in 1, 2, 3', {test: '1'})).toBeTruthy();
    expect(LabelSelector.match('test in 1, 2, 3', {test: '4'})).toBeFalsy();
    expect(LabelSelector.match('test in 1, 2, 3', {test1: '1'})).toBeFalsy();
});

test('notIn', () => {
    expect(LabelSelector.match('test notin 1, 2, 3', {test: '1'})).toBeFalsy();
    expect(LabelSelector.match('test notin 1, 2, 3', {test: '4'})).toBeTruthy();
    expect(LabelSelector.match('test notin 1, 2, 3', {test1: '1'})).toBeTruthy();
});

test('equal', () => {
    expect(LabelSelector.match('test=hello', {test: 'hello'})).toBeTruthy();
    expect(LabelSelector.match('test=world', {test: 'hello'})).toBeFalsy();
    expect(LabelSelector.match('test==hello', {test: 'hello'})).toBeTruthy();
});

test('notEqual', () => {
    expect(LabelSelector.match('test!=hello', {test: 'hello'})).toBeFalsy();
    expect(LabelSelector.match('test!=world', {test: 'hello'})).toBeTruthy();
});

test('greaterThen', () => {
    expect(LabelSelector.match('test gt 1', {test: '2'})).toBeTruthy();
    expect(LabelSelector.match('test gt 3', {test: '2'})).toBeFalsy();
});

test('lessThen', () => {
    expect(LabelSelector.match('test lt 1', {test: '2'})).toBeFalsy();
    expect(LabelSelector.match('test lt 3', {test: '2'})).toBeTruthy();
});

describe('toKubernetesSelector', () => {
    test.each([
        ['team', 'team'],
        ['!team', '!team'],
        ['app.kubernetes.io/instance', 'app.kubernetes.io/instance'],
        ['team=core', 'team=core'],
        ['team == core', 'team==core'],
        ['team!=core', 'team!=core'],
        ['example.com/team=core', 'example.com/team=core'],
        ['env in prod, staging', 'env in (prod,staging)'],
        ['env notin prod,staging', 'env notin (prod,staging)']
    ])('converts %p to %p', (selector, expected) => {
        expect(LabelSelector.toKubernetesSelector(selector)).toBe(expected);
    });

    test.each([
        // Kubernetes compares integers, match() compares floats.
        ['replicas gt 1'],
        ['replicas lt 1'],
        // match() only compares the first value.
        ['team=core,ops'],
        // The API server would reject these as invalid label keys or values.
        ['team name=core'],
        ['team=core team'],
        ['Example.com/team=core'],
        ['-team'],
        ['team=-core'],
        [`team=${'a'.repeat(64)}`],
        [`${'a'.repeat(254)}/team`],
        ['env in (prod,staging)'],
        ['']
    ])('does not convert %p', selector => {
        expect(LabelSelector.toKubernetesSelector(selector)).toBeUndefined();
    });

    // Every converted selector must keep match()'s meaning, including for labels that are missing.
    test.each([
        ['team=core', {team: 'core'}, true],
        ['team!=core', {}, true],
        ['env notin prod', {}, true],
        ['env in prod', {}, false],
        ['!team', {team: 'core'}, false]
    ])('match(%p, %p) is %p, as Kubernetes evaluates it', (selector, labels, expected) => {
        expect(LabelSelector.toKubernetesSelector(selector)).toBeDefined();
        expect(LabelSelector.match(selector, labels)).toBe(expected);
    });
});
