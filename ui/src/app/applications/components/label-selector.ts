type operatorFn = (labels: {[name: string]: string}, key: string, values: string[]) => boolean;

const operators: {[type: string]: operatorFn} = {
    '!=': (labels, key, values) => labels[key] !== values[0],
    '==': (labels, key, values) => labels[key] === values[0],
    '=': (labels, key, values) => labels[key] === values[0],
    'notin': (labels, key, values) => !values.includes(labels[key]),
    'in': (labels, key, values) => values.includes(labels[key]),
    'gt': (labels, key, values) => parseFloat(labels[key]) > parseFloat(values[0]),
    'lt': (labels, key, values) => parseFloat(labels[key]) < parseFloat(values[0])
};

function split(input: string, delimiter: string): string[] {
    return input
        .split(delimiter)
        .map(part => part.trim())
        .filter(part => part !== '');
}

export type LabelSelector = (labels: {[name: string]: string}) => boolean;

export function parse(selector: string): LabelSelector {
    for (const type of Object.keys(operators)) {
        const operator = operators[type];
        const parts = split(selector, type);
        if (parts.length > 1) {
            const values = split(parts[1], ',');
            return labels => operator(labels, parts[0], values);
        }
    }
    if (selector.startsWith('!')) {
        return labels => !labels.hasOwnProperty(selector.slice(1));
    }
    return labels => labels.hasOwnProperty(selector);
}

export function match(selector: string, labels: {[name: string]: string}): boolean {
    return parse(selector)(labels || {});
}
