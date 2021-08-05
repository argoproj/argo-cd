import * as React from 'react';
import { FormApi, NestedForm, Text, Form } from 'react-form';
import { Checkbox, FormField } from 'argo-ui';
import { NumberField } from '../../../shared/components';
import * as models from '../../../shared/models';

require('./application-retry-options.scss');



function buildFormItem(label: string, propertyPath: string, component: React.ComponentType, formApi: FormApi, componentProps?: Record<string, any>) {
    return <FormField
        formApi={formApi}
        label={label}
        field={propertyPath}
        component={component}
        componentProps={componentProps}
    />
}

const onlyPositiveValidation = {
    min: '1',
    step: '1'
}

const retryOptions: Array<(formApi: FormApi) => React.ReactNode> = [
    formApi => buildFormItem('Limit', 'limit', NumberField, formApi, onlyPositiveValidation),
    formApi => buildFormItem('Duration', 'backoff.duration', Text, formApi),
    formApi => buildFormItem('Max Duration', 'backoff.maxDuration', Text, formApi),
    formApi => buildFormItem('Factor', 'backoff.factor', NumberField, formApi, onlyPositiveValidation),
];

const regex = /\b((\d+(\.\d+)?)\s*(h|hr|hrs?|hours?))?(\s*(\d+)\s*(m|min|mins?|minutes?))\b/m

export const ApplicationRetryOptions = ({ formApi, initValues }: { formApi: FormApi, initValues?: models.RetryStrategy }) => {
    const [retry, setRetry] = React.useState(!!initValues)


    const toggleRetry = (value: boolean) => {

        if (!value) {
            const formState = formApi.getFormState()
            const values = formState.values
            const errors = formState.errors

            const { ['retryStrategy']: delVal, ...newValues } = values
            const { ['retryStrategy']: delErr, ...newErrors } = errors
        
            formApi.setFormState({
                ...formState,
                values: { ...newValues },
                errors: { ...newErrors }
            })
        }

        setRetry(value)
    }


    return <div style={{ marginBottom: '1em' }}>
        <Checkbox id="retry" checked={retry} onChange={(val) => toggleRetry(val)} />
        <label htmlFor="retry">Retry</label>
        {
            retry &&
            <NestedForm field="retryStrategy">
                <Form
                    defaultValues={{ ...initValues }}
                    validateError={(values) => {
                        const isRetryEnabled = () => !!values && values.backoff
                        const getBackoffSafe = () => values.backoff || {}

                        return {
                            'limit': (
                                isRetryEnabled() &&
                                !values.limit &&
                                values.hasOwnProperty('limit')
                            )
                                && 'Limit is required',

                            'backoff.duration': (
                                (isRetryEnabled() && getBackoffSafe().hasOwnProperty('duration')) &&
                                (
                                    (!getBackoffSafe().duration && 'Duration is required') ||
                                    (!regex.test(getBackoffSafe().duration) && 'Should be 10m/1h10m')
                                )
                            ),

                            'backoff.maxDuration': (
                                (isRetryEnabled() && getBackoffSafe().hasOwnProperty('maxDuration')) &&
                                (
                                    (!getBackoffSafe().maxDuration && 'Max Duration is required') ||
                                    (!regex.test(getBackoffSafe().maxDuration) && 'Should be 10m/1h10m')
                                )
                            ),

                            'backoff.factor': (
                                (isRetryEnabled() && getBackoffSafe().hasOwnProperty('factor')) &&
                                (
                                    (!getBackoffSafe().factor && 'Factor is required')
                                )
                            )
                        }
                    }}
                >
                    {(nestedFormApi) => {
                        return (
                            <div className='row application-retry-options'>
                                {retryOptions.map((render, i) => (
                                    <div
                                        className="columns small-6 application-retry-options__item"
                                        key={i}
                                    >
                                        {render(nestedFormApi)}
                                    </div>
                                ))}
                            </div>
                        )
                    }
                    }
                </Form>
            </NestedForm>
        }
    </div>
}