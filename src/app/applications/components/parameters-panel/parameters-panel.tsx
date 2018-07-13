import * as React from 'react';

import * as models from '../../../shared/models';

import { getParamsWithOverridesInfo } from '../utils';

export const ParametersPanel = (props: { params: models.ComponentParameter[], overrides?: models.ComponentParameter[]}) => {
    const componentParams = getParamsWithOverridesInfo(props.params, props.overrides);
    return (
        <div className='white-box'>
            <div className='white-box__details'>
            {Array.from(componentParams.keys()).map((component) => (
                componentParams.get(component).map((param, i) => (
                    [i === 0 && <p key={component + param.name + 'header'}>
                        {component.toUpperCase()}
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
