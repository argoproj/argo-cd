/* eslint-disable no-prototype-builtins */
import * as React from 'react';
import {FormApi, NestedForm, Text, Form} from 'react-form';
import {Checkbox, FormField} from 'argo-ui';
import {omit} from 'lodash-es';
import {NumberField} from '../../../shared/components';
import * as models from '../../../shared/models';

import './application-retry-options.scss';

// eslint-disable-next-line no-useless-escape
const durationRegex = /^([\d\.]+[HMS])+$/i;
const durationRegexError = 'Should be 1h10m10s/10h10m/10m/10s';

const onlyPositiveValidation = {
    min: '1',
    step: '1'
};

function buildFormItem(label: string, propertyPath: string, component: React.ComponentType, formApi: FormApi, componentProps?: Record<string, any>) {
    return <FormField formApi={formApi} label={label} field={propertyPath} component={component} componentProps={componentProps} />;
}

const retryOptions: Array<(formApi: FormApi) => React.ReactNode> = [
    formApi => buildFormItem('Limit', 'limit', NumberField, formApi, onlyPositiveValidation),
    formApi => buildFormItem('Duration', 'backoff.duration', Text, formApi),
    formApi => buildFormItem('Max Duration', 'backoff.maxDuration', Text, formApi),
    formApi => buildFormItem('Factor', 'backoff.factor', NumberField, formApi, onlyPositiveValidation)
];

const defaultInitialValues = {
    limit: 2,
    backoff: {
        duration: '5s',
        maxDuration: '3m0s',
        factor: 2
    }
};

export const ApplicationRetryForm = ({initValues, field = 'retryStrategy'}: {initValues?: models.RetryStrategy; field: string}) => {
    return (
        <NestedForm field={field}>
            <Form
                defaultValues={{
                    ...defaultInitialValues,
                    ...initValues
                }}
                validateError={values => {
                    const backoff = values.backoff || {};

                    if (!values) {
                        return {};
                    }

                    return {
                        'limit': !values.limit && values.hasOwnProperty('limit') && 'Limit is required',

                        'backoff.duration':
                            backoff.hasOwnProperty('duration') && ((!backoff.duration && 'Duration is required') || (!durationRegex.test(backoff.duration) && durationRegexError)),

                        'backoff.maxDuration':
                            backoff.hasOwnProperty('maxDuration') &&
                            ((!backoff.maxDuration && 'Max Duration is required') || (!durationRegex.test(backoff.maxDuration) && durationRegexError)),

                        'backoff.factor': backoff.hasOwnProperty('factor') && !backoff.factor && 'Factor is required'
                    };
                }}>
                {nestedFormApi => {
                    return (
                        <div className='row application-retry-options-list'>
                            {retryOptions.map((render, i) => (
                                <div className='columns small-6 application-retry-options-list__item' key={i}>
                                    {render(nestedFormApi)}
                                </div>
                            ))}
                        </div>
                    );
                }}
            </Form>
        </NestedForm>
    );
};

export const ApplicationRetryOptions = ({
    formApi,
    initValues,
    field = 'retryStrategy',
    retry,
    setRetry,
    id
}: {
    formApi: FormApi;
    field?: string;
    initValues?: models.RetryStrategy;
    retry?: boolean;
    setRetry?: (value: boolean) => any;
    id?: string;
}) => {
    const [retryInternal, setRetryInternal] = React.useState(!!initValues);

    const toggleRetry = (value: boolean) => {
        if (!value) {
            const formState = formApi.getFormState();
            const values = formState.values;
            const errors = formState.errors;

            const newValues = omit(values, field);
            const newErrors = omit(errors, field);

            formApi.setFormState({
                ...formState,
                values: newValues,
                errors: newErrors
            });
        }
        if (setRetry != null) {
            setRetry(value);
        } else {
            setRetryInternal(value);
        }
    };
    const isChecked = setRetry != null ? retry : retryInternal;
    return (
        <div className='application-retry-options'>
            <Checkbox id={`retry-${id}`} checked={isChecked} onChange={val => toggleRetry(val)} />
            <label htmlFor={`retry-${id}`}>Retry</label>
            {isChecked && <ApplicationRetryForm initValues={initValues} field={field} />}
        </div>
    );
};
