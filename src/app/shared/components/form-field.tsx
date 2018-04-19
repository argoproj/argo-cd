import * as classNames from 'classnames';
import * as React from 'react';
import { WrappedFieldProps } from 'redux-form';

export const FormField = (props: WrappedFieldProps & { type: string }) => {
    return (
        <div>
            <input {...props.input} className={classNames('argo-field', {
                'argo-has-value': props.input.value,
            })} type={props.type} />
            <label className='argo-label-placeholder'>{props.label}</label>
            {props.meta.touched &&
                (props.meta.error && <div className='argo-form-row__error-msg'>{props.meta.error}</div>)
            }
        </div>
    );
};
