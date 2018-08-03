import { Select as ArgoSelect, SelectOption, SelectProps } from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import * as ReactForm from 'react-form';

require('./form-field.scss');

export const FormField: <E, T extends ReactForm.FieldProps & {className?: string}>(
    props: React.Props<E> & {
    label: string,
    field: string,
    formApi: ReactForm.FormApi,
    component: React.ComponentType<T>,
    componentProps?: T,
}) => React.ReactElement<E> = (props) => {

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

export const Select = ReactForm.FormField((props: SelectProps & { fieldApi: ReactForm.FieldApi, placeholder?: string, className?: string }) => {
    const { fieldApi: {getValue, setValue}, onChange, ...rest } = props;
    const value = getValue();

    return (
        <div className={classNames(props.className, 'form-field__select')}>
            <ArgoSelect {...rest} value={!value && value !== 0 ? '' : value} placeholder={props.placeholder}
                onChange={(option) => {
                    setValue(option.value);
                }
            }/>
        </div>
    );
}) as React.ComponentType<ReactForm.FieldProps & { options: (SelectOption | string)[], placeholder?: string, className?: string }>;
