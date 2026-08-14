import * as React from 'react';
import {ReactForm} from 'argo-ui';

export const parseNumberFieldValue = (value: string): number | undefined => (value === '' ? undefined : parseInt(value, 10));

export const NumberField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi; className: string; onBlur?: () => void}) => {
    const {
        fieldApi: {getValue, setValue, setTouched},
        onBlur = () => setTouched(true),
        ...rest
    } = props;

    return <input {...rest} className={props.className} type='number' value={getValue()} onChange={el => setValue(parseNumberFieldValue(el.target.value))} onBlur={onBlur} />;
});
