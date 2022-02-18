import * as React from 'react';
import * as ReactForm from 'react-form';

export const NumberField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi; className: string; onBlur?: () => void}) => {
    const {
        fieldApi: {getValue, setValue, setTouched},
        onBlur = () => setTouched(true),
        ...rest
    } = props;

    return <input {...rest} className={props.className} type='number' value={getValue()} onChange={el => setValue(parseInt(el.target.value, 10))} onBlur={onBlur} />;
});
