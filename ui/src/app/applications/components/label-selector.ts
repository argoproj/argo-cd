/* eslint-disable no-prototype-builtins */
type operatorFn = (labels: {[name: string]: string}, key: string, values: string[]) => boolean;

const operators: {[type: string]: operatorFn} = {
    '!=': (labels, key, values) => labels[key] !== values[0],
    '==': (labels, key, values) => labels[key] === values[0],
    '=': (labels, key, values) => labels[key] === values[0],
    '[\\W]notin[\\W]': (labels, key, values) => !values.includes(labels[key]),
    '[\\W]in[\\W]': (labels, key, values) => values.includes(labels[key]),
    '[\\W]gt[\\W]': (labels, key, values) => parseFloat(labels[key]) > parseFloat(values[0]),
    '[\\W]lt[\\W]': (labels, key, values) => parseFloat(labels[key]) < parseFloat(values[0])
};

function split(input: string, delimiter: string | RegExp): string[] {
    return input
        .split(delimiter)
        .map(part => part.trim())
        .filter(part => part !== '');
}

export type LabelSelector = (labels: {[name: string]: string}) => boolean;

type Requirement = {operator: string; key: string; values: string[]} | {operator: 'exists' | '!exists'; key: string};

function parseRequirement(selector: string): Requirement {
    for (const operator of Object.keys(operators)) {
        const parts = split(selector, new RegExp(operator));
        if (parts.length > 1) {
            return {operator, key: parts[0], values: split(parts[1], ',')};
        }
    }
    if (selector.startsWith('!')) {
        return {operator: '!exists', key: selector.slice(1)};
    }
    return {operator: 'exists', key: selector};
}

export function parse(selector: string): LabelSelector {
    const req = parseRequirement(selector);
    switch (req.operator) {
        case 'exists':
            return labels => labels.hasOwnProperty(req.key);
        case '!exists':
            return labels => !labels.hasOwnProperty(req.key);
        default:
            return labels => operators[req.operator](labels, req.key, (req as {values: string[]}).values);
    }
}

// Kubernetes label syntax: an optional DNS subdomain prefix and a name for keys, a name for values.
const labelName = '[A-Za-z0-9]([-A-Za-z0-9_.]{0,61}[A-Za-z0-9])?';
const labelKeyRegex = new RegExp(`^(([a-z0-9]([-a-z0-9]*[a-z0-9])?)(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?${labelName}$`);
const labelValueRegex = new RegExp(`^${labelName}$`);

/**
 * Converts a selector into the Kubernetes label selector syntax the API server accepts, or returns
 * undefined when the API server would match differently from match(). Callers can send the result to
 * the server to narrow what it returns, but must keep matching on the client: a selector that can't be
 * converted is only applied there.
 *
 * gt and lt are left out: Kubernetes compares integers, while match() compares floats.
 */
export function toKubernetesSelector(selector: string): string | undefined {
    const req = parseRequirement(selector);
    const prefix = req.key.includes('/') ? req.key.slice(0, req.key.indexOf('/')) : '';
    if (!labelKeyRegex.test(req.key) || prefix.length > 253) {
        return undefined;
    }
    if (req.operator === 'exists') {
        return req.key;
    }
    if (req.operator === '!exists') {
        return `!${req.key}`;
    }
    const values = (req as {values: string[]}).values;
    // Kubernetes reads an empty set, `key in ()`, as the set holding the empty string.
    if (values.length === 0 || !values.every(value => labelValueRegex.test(value))) {
        return undefined;
    }
    switch (req.operator) {
        case '=':
        case '==':
        case '!=':
            // match() compares against the first value only, so a list would mean something different.
            return values.length === 1 ? `${req.key}${req.operator}${values[0]}` : undefined;
        case '[\\W]in[\\W]':
            return `${req.key} in (${values.join(',')})`;
        case '[\\W]notin[\\W]':
            return `${req.key} notin (${values.join(',')})`;
        default:
            return undefined;
    }
}

export function match(selector: string, labels: {[name: string]: string}): boolean {
    return parse(selector)(labels || {});
}
