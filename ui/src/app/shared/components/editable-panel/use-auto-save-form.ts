import {FormApi, FormState} from 'argo-ui';
import {useCallback, useEffect, useRef} from 'react';

type Save<T> = (input: T, query: {validate?: boolean}) => Promise<any>;

function serializeValues(values: unknown): string {
    return JSON.stringify(values);
}

export function useAutoSaveForm<T extends {}>(values: T, noReadonlyMode?: boolean, save?: Save<T>) {
    const formApiRef = useRef<FormApi | null>(null);
    // Form.formDidUpdate also runs after programmatic value synchronization and
    // touched/error updates. Track the last inbound or saved value snapshot so
    // only a new user value is propagated to the parent form.
    const lastHandledValuesRef = useRef(serializeValues(values));

    useEffect(() => {
        const valuesString = serializeValues(values);
        lastHandledValuesRef.current = valuesString;

        if (noReadonlyMode && formApiRef.current && serializeValues(formApiRef.current.values) !== valuesString) {
            formApiRef.current.setAllValues(values);
        }
    }, [values, noReadonlyMode]);

    const onFormDidUpdate = useCallback(
        async (form: FormState) => {
            if (!noReadonlyMode || !save) {
                return;
            }

            const valuesString = serializeValues(form.values);
            if (valuesString === lastHandledValuesRef.current) {
                return;
            }

            lastHandledValuesRef.current = valuesString;
            await save(form.values as T, {});
        },
        [noReadonlyMode, save]
    );

    return {formApiRef, onFormDidUpdate};
}
