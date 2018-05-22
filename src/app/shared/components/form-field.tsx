import * as classNames from 'classnames';
import * as React from 'react';
import { FieldProps, FormApi } from 'react-form';

export const FormField = (props: React.Props<any> & {
    label: string,
    field: string,
    formApi: FormApi,
    component: React.StatelessComponent<FieldProps & React.InputHTMLAttributes<any>>,
    componentProps?: React.InputHTMLAttributes<any>,
}) => {
    return (
        <div>
            <props.component
                {...props.componentProps || {}}
                field={props.field}
                className={classNames({ 'argo-field': true, 'argo-has-value': !!props.formApi.values[props.field] })}/>

            <label className='argo-label-placeholder'>{props.label}</label>
            {props.formApi.touched[props.field] &&
                (props.formApi.errors[props.field] && <div className='argo-form-row__error-msg'>{props.formApi.errors[props.field]}</div>)
            }
        </div>
    );
};
