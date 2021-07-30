import * as React from 'react';
import {FormApi, Text} from 'react-form';
import {FormField} from 'argo-ui';
import {NumberField} from '../../../shared/components';

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
    min:'1',
    step:'1'
}

const retryOptions: Array<(formApi: FormApi) => React.ReactNode> = [
    formApi => buildFormItem('Limit', 'retryStrategy.limit', NumberField, formApi, onlyPositiveValidation),
    formApi => buildFormItem('Duration', 'retryStrategy.backoff.duration', Text, formApi),
    formApi => buildFormItem('Max Duration', 'retryStrategy.backoff.maxDuration', Text, formApi),
    formApi => buildFormItem('Factor', 'retryStrategy.backoff.factor', NumberField, formApi, onlyPositiveValidation),
];


export const ApplicationRetryOptions = ({ formApi }: { formApi: FormApi }) => {

    return <div className='row application-retry-options'>
        {retryOptions.map((render, i) => (
            <div
                className="columns small-6 application-retry-options__item"
                key={i}
            >
                {render(formApi)}
            </div>
        ))}
    </div>
}