import {Checkbox} from 'argo-ui';
import * as React from 'react';
import * as ReactForm from 'react-form';

export const CheckboxField = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi; id: string; className: string; checked: boolean}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;

    return <Checkbox id={props.id} checked={!!getValue()} onChange={setValue} />;
});
