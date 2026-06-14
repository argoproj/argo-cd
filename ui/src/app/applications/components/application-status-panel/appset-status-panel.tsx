import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {ARGO_GRAY6_COLOR} from '../../../shared/components';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {getAppSetConditionCategory, getAppSetHealthStatus, HealthStatusIcon} from '../utils';

import './application-status-panel.scss';

interface Props {
    appSet: models.ApplicationSet;
    showConditions?: () => any;
}

interface SectionInfo {
    title: string;
    helpContent?: string;
}

const sectionLabel = (info: SectionInfo) => (
    <label style={{display: 'flex', alignItems: 'flex-start', fontSize: '12px', fontWeight: 600, color: ARGO_GRAY6_COLOR, minHeight: '18px'}}>
        {info.title}
        {info.helpContent && (
            <span style={{marginLeft: '5px'}}>
                <HelpIcon title={info.helpContent} />
            </span>
        )}
    </label>
);

const getConditionCounts = (conditions: models.ApplicationSetCondition[]) => {
    const counts = {info: 0, warning: 0, error: 0};
    if (!conditions) return counts;

    conditions.forEach(c => {
        const category = getAppSetConditionCategory(c);
        counts[category]++;
    });
    return counts;
};

export const ApplicationSetStatusPanel = ({appSet, showConditions}: Props) => {
    const healthStatus = getAppSetHealthStatus(appSet);
    const conditions = appSet.status?.conditions || [];
    const conditionCounts = getConditionCounts(conditions);
    const latestCondition = conditions.length > 0 ? conditions[conditions.length - 1] : null;

    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item'>
                {sectionLabel({title: 'APPSET HEALTH', helpContent: 'The health status of your ApplicationSet derived from its conditions'})}
                <div className='application-status-panel__item-value'>
                    <HealthStatusIcon state={{status: healthStatus, message: ''}} />
                    &nbsp;
                    {healthStatus}
                </div>
                {latestCondition?.message && <div className='application-status-panel__item-name'>{latestCondition.message}</div>}
            </div>

            {conditions.length > 0 && (
                <div className='application-status-panel__item'>
                    {sectionLabel({title: 'CONDITIONS'})}
                    <div className='application-status-panel__item-value application-status-panel__conditions' onClick={() => showConditions && showConditions()}>
                        {conditionCounts.info > 0 && (
                            <a className='info'>
                                <i className='fa fa-info-circle application-status-panel__item-value__status-button' /> {conditionCounts.info} Info
                            </a>
                        )}
                        {conditionCounts.error > 0 && (
                            <a className='error'>
                                <i className='fa fa-exclamation-circle application-status-panel__item-value__status-button' /> {conditionCounts.error} Error
                                {conditionCounts.error !== 1 && 's'}
                            </a>
                        )}
                    </div>
                </div>
            )}

            {latestCondition?.lastTransitionTime && (
                <div className='application-status-panel__item'>
                    {sectionLabel({title: 'LAST UPDATED'})}
                    <div className='application-status-panel__item-value'>
                        <Timestamp date={latestCondition.lastTransitionTime} />
                    </div>
                </div>
            )}
        </div>
    );
};
