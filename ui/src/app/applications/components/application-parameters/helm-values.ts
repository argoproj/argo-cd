import * as jsYaml from 'js-yaml';

export function parseHelmValues(values: string | undefined): {value?: any; error?: string} {
    if (values === undefined) {
        return {error: 'Values must be valid YAML'};
    }
    try {
        return {value: jsYaml.load(values)};
    } catch {
        return {error: 'Values must be valid YAML'};
    }
}

export function validateHelmValues(values: string | undefined) {
    if (!values) {
        return undefined;
    }
    const parsedValues = parseHelmValues(values);
    const isMap = parsedValues.value !== null && typeof parsedValues.value === 'object' && !Array.isArray(parsedValues.value);
    return parsedValues.error || (isMap ? undefined : 'Values must be a map');
}
