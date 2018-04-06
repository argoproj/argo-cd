import * as React from 'react';

import * as models from '../../../shared/models';

export const ParametersPanel = (props: { params: models.ComponentParameter[], overrides?: models.ComponentParameter[]}) => {
    const componentParams = new Map<string, (models.ComponentParameter & {original: string})[]>();
    (props.params || []).map((param) => {
        const override = (props.overrides || []).find((item) => item.component === param.component && item.name === param.name);
        const res = {...param, original: ''};
        if (override) {
            res.original = res.value;
            res.value = override.value;
        }
        return res;
    }).forEach((param) => {
        const params = componentParams.get(param.component) || [];
        params.push(param);
        componentParams.set(param.component, params);
    });
    return (
        <div className='white-box'>
            <div className='white-box__details'>
            {Array.from(componentParams.keys()).map((component) => (
                componentParams.get(component).map((param, i) => (
                    [<p key={component + param.name + 'header'}>
                        {i === 0 && component.toUpperCase()}
                    </p>,
                    <div className='row white-box__details-row' key={component + param.name}>
                        <div className='columns small-2'>
                            {param.name}:
                        </div>
                        <div className='columns small-10'>
                            <span title={param.value}>
                                {param.original && <span className='fa fa-exclamation-triangle' title={`Original value: ${param.original}`}/>}
                                {param.value}
                            </span>
                        </div>
                    </div>]
                ))
            ))}
            </div>
        </div>
    );
};
