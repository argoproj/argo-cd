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
