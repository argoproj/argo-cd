import * as jsYaml from 'js-yaml';

/**
 * Parses the raw YAML of the Helm VALUES editor and returns a validation error, if any.
 *
 * The result feeds the form's `validateError` callback, which is invoked from a React effect.
 * An exception thrown there is not caught by the form and unmounts the whole application, so
 * this must never throw: malformed YAML (a duplicated mapping key, for example) is reported as
 * a field error so the user can keep editing.
 */
export function helmValuesError(rawValues: unknown): string | null {
    const text = typeof rawValues === 'string' ? rawValues : '';
    // An empty document is valid: the chart defaults apply. js-yaml 5 rejects it, so short-circuit.
    if (text.trim() === '') {
        return null;
    }
    try {
        const parsed = jsYaml.load(text);
        return typeof parsed === 'object' ? null : 'Values must be a map';
    } catch (error) {
        const reason = (error as {reason?: string}).reason || (error as Error).message?.split('\n')[0] || 'unknown error';
        return `Values are not valid YAML: ${reason}`;
    }
}
