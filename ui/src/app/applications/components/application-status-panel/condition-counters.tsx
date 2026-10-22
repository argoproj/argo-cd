import * as React from 'react';

export const ConditionCounters = ({infos, warnings, errors}: {infos?: number; warnings?: number; errors?: number}) => (
    <React.Fragment>
        {!!infos && (
            <a className='info'>
                <i className='fa fa-info-circle application-status-panel__item-value__status-button' />
                <span className='sync-condition-details'>{infos} Info</span>
            </a>
        )}
        {!!warnings && (
            <a className='warning'>
                <i className='fa fa-exclamation-triangle application-status-panel__item-value__status-button' />
                <span className='sync-condition-details'>
                    {warnings} Warning{warnings !== 1 && 's'}
                </span>
            </a>
        )}
        {!!errors && (
            <a className='error'>
                <i className='fa fa-exclamation-circle application-status-panel__item-value__status-button' />
                <span className='sync-condition-details'>
                    {errors} Error{errors !== 1 && 's'}
                </span>
            </a>
        )}
    </React.Fragment>
);
