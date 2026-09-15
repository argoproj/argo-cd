import * as jsYaml from 'js-yaml';

const jsonMergePatch = require('json-merge-patch');

function deepClone<T>(input: T): T {
    return JSON.parse(JSON.stringify(input)) as T;
}

export function cloneAndDumpYaml<T>(input: T): {snapshot: T; yaml: string} {
    const snapshot = deepClone(input);
    return {snapshot, yaml: jsYaml.dump(snapshot)};
}

export function dumpYaml(input: unknown): string {
    return input ? jsYaml.dump(input) : '';
}

export function buildYamlMergePatch(base: unknown, yamlText: string): object {
    const updated = jsYaml.load(yamlText);
    return jsonMergePatch.generate(base, updated) || {};
}

export function isEmptyPatch(patch: object | null): boolean {
    return !patch || Object.keys(patch).length === 0;
}
