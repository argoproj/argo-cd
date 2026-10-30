import {Checkbox} from 'argo-ui';
import * as React from 'react';
import {ReactForm} from 'argo-ui';

export const CheckboxField = ReactForm.FormField(
    (props: {fieldApi: ReactForm.FieldApi; id: string; className: string; checked: boolean; onChange?: (checked: boolean) => void}) => {
        const {
            fieldApi: {getValue, setValue}
        } = props;

        return (
            <Checkbox
                id={props.id}
                checked={!!getValue()}
                onChange={checked => {
                    setValue(checked);
                    props.onChange?.(checked);
                }}
            />
        );
    }
);
