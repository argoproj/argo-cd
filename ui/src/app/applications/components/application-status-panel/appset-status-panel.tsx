import {HelpIcon} from 'argo-ui';
import * as React from 'react';
import {ARGO_GRAY6_COLOR} from '../../../shared/components';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {getAppSetConditionCategory, getAppSetHealthStatus, getHealthStatusColor, HealthStatusIcon} from '../utils';
import {ConditionCounters} from './condition-counters';

import './application-status-panel.scss';

interface Props {
    appSet: models.ApplicationSet;
    collapsed?: boolean;
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

export const ApplicationSetStatusPanel = ({appSet, collapsed, showConditions}: Props) => {
    const healthStatus = getAppSetHealthStatus(appSet);
    const conditions = appSet.status?.conditions || [];
    const conditionCounts = getConditionCounts(conditions);
    const latestCondition = conditions.length > 0 ? conditions[conditions.length - 1] : null;
    // Unknown's palette color is too low-contrast for text; inherit the themed color there
    const healthTextColor = healthStatus === 'Unknown' ? undefined : getHealthStatusColor(healthStatus);

    if (collapsed) {
        return (
            <div className='application-status-panel application-status-panel--collapsed row'>
                <div className='application-status-panel__collapsed-item' title='AppSet Health' style={{color: healthTextColor}}>
                    <HealthStatusIcon state={{status: healthStatus, message: ''}} />
                    &nbsp;
                    {healthStatus}
                </div>
                {conditions.length > 0 && (
                    <div className='application-status-panel__collapsed-item application-status-panel__conditions' onClick={() => showConditions && showConditions()}>
                        <ConditionCounters infos={conditionCounts.info} warnings={conditionCounts.warning} errors={conditionCounts.error} />
                    </div>
                )}
            </div>
        );
    }

    return (
        <div className='application-status-panel row'>
            <div className='application-status-panel__item'>
                {sectionLabel({title: 'APPSET HEALTH', helpContent: 'The health status of your ApplicationSet derived from its conditions'})}
                <div className='application-status-panel__item-value' style={{color: healthTextColor}}>
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
                        <ConditionCounters infos={conditionCounts.info} warnings={conditionCounts.warning} errors={conditionCounts.error} />
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
