import {Checkbox} from 'argo-ui';
import * as React from 'react';
import * as ReactForm from 'react-form';

import {ArrayInput, NameValueEditor} from '../../../shared/components';

export interface Var {
    name: string;
    value: string;
    code: boolean;
}

const VarInputEditor = (item: Var, onChange: (item: Var) => any) => (
    <React.Fragment>
        {NameValueEditor(item, onChange)}
        &nbsp;
        <Checkbox checked={!!item.code} onChange={val => onChange({...item, code: val})} />
        &nbsp;
    </React.Fragment>
);

export const VarsInputField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue, setTouched}
    } = props;
    const val = getValue() || [];
    return (
        <ArrayInput
            editor={VarInputEditor}
            items={val}
            onChange={items => {
                setTouched(true);
                setValue(items);
            }}
        />
    );
});
