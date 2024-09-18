import {Checkbox, DropDown, Duration, NotificationType, Ticker, HelpIcon} from 'argo-ui';
import * as moment from 'moment';
import * as PropTypes from 'prop-types';
import * as React from 'react';

import {ErrorNotification, Revision, Timestamp} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import * as utils from '../utils';

import './application-operation-state.scss';

interface Props {
    application: models.Application;
    operationState: models.OperationState;
}
const buildResourceUniqueId = (res: Omit<models.ResourceRef, 'uid'>) => `${res.group}-${res.kind}-${res.version}-${res.namespace}-${res.name}`;

const Filter = (props: {filters: string[]; setFilters: (f: string[]) => void; options: string[]; title: string; style?: React.CSSProperties}) => {
    const {filters, setFilters, options, title, style} = props;
    return (
        <DropDown
            isMenu={true}
            anchor={() => (
                <div title='Filter' style={style}>
                    <button className='argo-button argo-button--base'>
                        {title} <i className='argo-icon-filter' aria-hidden='true' />
                    </button>
                </div>
            )}>
            {options.map(f => (
                <div key={f} style={{minWidth: '150px', lineHeight: '2em', padding: '5px'}}>
                    <Checkbox
                        checked={filters.includes(f)}
                        onChange={checked => {
                            const selectedValues = [...filters];
                            const idx = selectedValues.indexOf(f);
                            if (idx > -1 && !checked) {
                                selectedValues.splice(idx, 1);
                            } else {
                                selectedValues.push(f);
                            }
                            setFilters(selectedValues);
                        }}
                    />
                    <label htmlFor={`filter__${f}`}>{f}</label>
                </div>
            ))}
        </DropDown>
    );
};

export const ApplicationOperationState: React.StatelessComponent<Props> = ({application, operationState}, ctx: AppContext) => {
    const operationAttributes = [
        {title: 'OPERATION', value: utils.getOperationType(application)},
        {title: 'PHASE', value: operationState.phase},
        ...(operationState.message ? [{title: 'MESSAGE', value: operationState.message}] : []),
        {title: 'STARTED AT', value: <Timestamp date={operationState.startedAt} />},
        {
            title: 'DURATION',
            value: (
                <Ticker>
                    {time => <Duration durationMs={((operationState.finishedAt && moment(operationState.finishedAt)) || time).diff(moment(operationState.startedAt)) / 1000} />}
                </Ticker>
            )
        }
    ];

    if (operationState.finishedAt && operationState.phase !== 'Running') {
        operationAttributes.push({title: 'FINISHED AT', value: <Timestamp date={operationState.finishedAt} />});
    } else if (operationState.phase !== 'Terminating') {
        operationAttributes.push({
            title: '',
            value: (
                <button
                    className='argo-button argo-button--base'
                    onClick={async () => {
                        const confirmed = await ctx.apis.popup.confirm('Terminate operation', 'Are you sure you want to terminate operation?');
                        if (confirmed) {
                            try {
                                await services.applications.terminateOperation(application.metadata.name, application.metadata.namespace);
                            } catch (e) {
                                ctx.apis.notifications.show({
                                    content: <ErrorNotification title='Unable to terminate operation' e={e} />,
                                    type: NotificationType.Error
                                });
                            }
                        }
                    }}>
                    Terminate
                </button>
            )
        });
    }
    if (operationState.syncResult) {
        operationAttributes.push({
            title: 'REVISION',
            value: (
                <div>
                    <Revision repoUrl={utils.getAppDefaultSource(application).repoURL} revision={utils.getAppDefaultOperationSyncRevision(application)} />
                    {utils.getAppDefaultOperationSyncRevisionExtra(application)}
                </div>
            )
        });
    }
    let initiator = '';
    if (operationState.operation.initiatedBy) {
        if (operationState.operation.initiatedBy.automated) {
            initiator = 'automated sync policy';
        } else {
            initiator = operationState.operation.initiatedBy.username;
        }
    }
    operationAttributes.push({title: 'INITIATED BY', value: initiator || 'Unknown'});

    const resultAttributes: {title: string; value: string}[] = [];
    const syncResult = operationState.syncResult;
    if (operationState.finishedAt) {
        if (syncResult) {
            (syncResult.resources || []).forEach(res => {
                resultAttributes.push({
                    title: `${res.namespace}/${res.kind}:${res.name}`,
                    value: res.message
                });
            });
        }
    }
    const [filters, setFilters] = React.useState([]);
    const [healthFilters, setHealthFilters] = React.useState([]);

    const Healths = Object.keys(models.HealthStatuses);
    const Statuses = Object.keys(models.ResultCodes);
    const OperationPhases = Object.keys(models.OperationPhases);
    // const syncPhases = ['PreSync', 'Sync', 'PostSync', 'SyncFail'];
    // const hookPhases = ['Running', 'Terminating', 'Failed', 'Error', 'Succeeded'];
    const resourceHealth = application.status.resources.reduce(
        (acc, res) => {
            if (res.health) {
                acc[buildResourceUniqueId(res)] = res.health;
            }

            return acc;
        },
        {} as Record<string, models.HealthStatus>
    );

    const combinedHealthSyncResult: models.SyncResourceResult[] = syncResult?.resources?.map(syncResultItem => {
        const uniqueResourceName = buildResourceUniqueId(syncResultItem);

        const healthStatus = resourceHealth[uniqueResourceName];

        const syncResultWithHealth: models.SyncResourceResult = {
            ...syncResultItem
        };

        if (healthStatus) {
            syncResultWithHealth.health = healthStatus;
        }

        return syncResultWithHealth;
    });
    let filtered: models.SyncResourceResult[] = [];

    if (combinedHealthSyncResult && combinedHealthSyncResult.length > 0) {
        filtered = combinedHealthSyncResult.filter(r => {
            if (filters.length === 0 && healthFilters.length === 0) {
                return true;
            }

            let pass = true;
            if (filters.length !== 0 && !filters.includes(getStatus(r))) {
                pass = false;
            }

            if (pass && healthFilters.length !== 0 && !healthFilters.includes(r.health?.status)) {
                pass = false;
            }

            return pass;
        });
    }

    return (
        <div>
            <div className='white-box'>
                <div className='white-box__details'>
                    {operationAttributes.map(attr => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>{attr.title}</div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>
            {syncResult && syncResult.resources && syncResult.resources.length > 0 && (
                <React.Fragment>
                    <div style={{display: 'flex'}}>
                        <label style={{display: 'block', marginBottom: '1em'}}>RESULT</label>
                        <div style={{marginLeft: 'auto'}}>
                            <Filter options={Healths} filters={healthFilters} setFilters={setHealthFilters} title='HEALTH' style={{marginRight: '5px'}} />
                            <Filter options={Statuses} filters={filters} setFilters={setFilters} title='STATUS' style={{marginRight: '5px'}} />
                            <Filter options={OperationPhases} filters={filters} setFilters={setFilters} title='HOOK' />
                        </div>
                    </div>
                    <div className='argo-table-list'>
                        <div className='argo-table-list__head'>
                            <div className='row'>
                                <div className='columns large-1 show-for-large application-operation-state__icons_container_padding'>KIND</div>
                                <div className='columns large-2 show-for-large'>NAMESPACE</div>
                                <div className='columns large-2 small-2'>NAME</div>
                                <div className='columns large-1 small-2'>STATUS</div>
                                <div className='columns large-1 small-2'>HEALTH</div>
                                <div className='columns large-1 show-for-large'>HOOK</div>
                                <div className='columns large-4 small-8'>MESSAGE</div>
                            </div>
                        </div>
                        {filtered.length > 0 ? (
                            filtered.map((resource, i) => (
                                <div className='argo-table-list__row' key={i}>
                                    <div className='row'>
                                        <div className='columns large-1 show-for-large application-operation-state__icons_container_padding'>
                                            <div className='application-operation-state__icons_container'>
                                                {resource.hookType && <i title='Resource lifecycle hook' className='fa fa-anchor' />}
                                            </div>
                                            <span title={getKind(resource)}>{getKind(resource)}</span>
                                        </div>
                                        <div className='columns large-2 show-for-large' title={resource.namespace}>
                                            {resource.namespace}
                                        </div>
                                        <div className='columns large-2 small-2' title={resource.name}>
                                            {resource.name}
                                        </div>
                                        <div className='columns large-1 small-2' title={getStatus(resource)}>
                                            <utils.ResourceResultIcon resource={resource} /> {getStatus(resource)}
                                        </div>
                                        <div className='columns large-1 small-2'>
                                            {resource.health ? (
                                                <div>
                                                    <utils.HealthStatusIcon state={resource?.health} /> {resource.health?.status}
                                                    {resource.health.message && <HelpIcon title={resource.health.message} />}
                                                </div>
                                            ) : (
                                                <>{'-'}</>
                                            )}
                                        </div>
                                        <div className='columns large-1 show-for-large' title={resource.hookType}>
                                            {resource.hookType}
                                        </div>
                                        <div className='columns large-4 small-8' title={resource.message}>
                                            <div className='application-operation-state__message'>{resource.message}</div>
                                        </div>
                                    </div>
                                </div>
                            ))
                        ) : (
                            <div style={{textAlign: 'center', marginTop: '2em', fontSize: '20px'}}>No Sync Results match filter</div>
                        )}
                    </div>
                </React.Fragment>
            )}
        </div>
    );
};

const getKind = (resource: models.ResourceResult): string => {
    return (resource.group ? `${resource.group}/${resource.version}` : resource.version) + `/${resource.kind}`;
};

const getStatus = (resource: models.ResourceResult): string => {
    return resource.hookType ? resource.hookPhase : resource.status;
};

ApplicationOperationState.contextTypes = {
    apis: PropTypes.object
};
