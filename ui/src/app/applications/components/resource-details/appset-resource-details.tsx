import {MockupList, Tab, Tabs} from 'argo-ui';
import * as React from 'react';
import {DataLoader, EventsList, Expandable, YamlEditor} from '../../../shared/components';
import {Timestamp} from '../../../shared/components/timestamp';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {Context} from '../../../shared/context';
import {ResourceIcon} from '../resource-icon';
import {ResourceLabel} from '../resource-label';
import {HealthStatusIcon, getAppSetHealthStatus, getAppSetConditionCategory} from '../utils';
import './resource-details.scss';

interface AppSetResourceDetailsProps {
    appSet: models.ApplicationSet;
}

export const AppSetResourceDetails = (props: AppSetResourceDetailsProps) => {
    const {appSet} = props;
    const appContext = React.useContext(Context);
    const tab = new URLSearchParams(appContext.history.location.search).get('tab');

    const spec = appSet.spec as any;
    const healthStatus = getAppSetHealthStatus(appSet);
    const conditions = appSet.status?.conditions || [];
    const conditionCounts = getConditionCounts(conditions);

    const getTabs = (): Tab[] => {
        const tabs: Tab[] = [
            {
                title: 'SUMMARY',
                key: 'summary',
                content: (
                    <div className='applicationset-summary'>
                        <div className='white-box'>
                            <div className='white-box__details'>
                                <p>{appSet.metadata.name.toLocaleUpperCase()}</p>
                                <SummaryItem title='NAMESPACE'>{appSet.metadata.namespace}</SummaryItem>
                                <SummaryItem title='CREATED AT'>{appSet.metadata.creationTimestamp && <Timestamp date={appSet.metadata.creationTimestamp} />}</SummaryItem>
                                <SummaryItem title='HEALTH'>
                                    <HealthStatusIcon state={{status: healthStatus, message: ''}} /> {healthStatus}
                                </SummaryItem>
                                {conditions.length > 0 && (
                                    <SummaryItem title='CONDITIONS'>
                                        <a onClick={() => appContext.navigation.goto('.', {conditions: 'true'}, {replace: true})}>
                                            {conditionCounts.error > 0 && (
                                                <span>
                                                    {conditionCounts.error} Error{conditionCounts.error !== 1 && 's'}
                                                </span>
                                            )}
                                            {conditionCounts.warning > 0 && (
                                                <span>
                                                    {' '}
                                                    {conditionCounts.warning} Warning{conditionCounts.warning !== 1 && 's'}
                                                </span>
                                            )}
                                            {conditionCounts.info > 0 && <span> {conditionCounts.info} Info</span>}
                                            {conditionCounts.error === 0 && conditionCounts.warning === 0 && conditionCounts.info === 0 && (
                                                <span>
                                                    {conditions.length} Condition{conditions.length !== 1 && 's'}
                                                </span>
                                            )}
                                        </a>
                                    </SummaryItem>
                                )}
                                <SummaryItem title='LABELS'>
                                    {Object.keys(appSet.metadata.labels || {}).length > 0
                                        ? Object.entries(appSet.metadata.labels)
                                              .map(([k, v]) => `${k}=${v}`)
                                              .join(' ')
                                        : ''}
                                </SummaryItem>
                                <SummaryItem title='ANNOTATIONS'>
                                    {Object.keys(appSet.metadata.annotations || {}).length > 0 ? (
                                        <Expandable height={48}>
                                            {Object.entries(appSet.metadata.annotations || {})
                                                .map(([k, v]) => `${k}=${v}`)
                                                .join(' ')}
                                        </Expandable>
                                    ) : (
                                        ''
                                    )}
                                </SummaryItem>
                            </div>
                        </div>

                        {spec?.syncPolicy && (
                            <div className='white-box'>
                                <div className='white-box__details'>
                                    <p>SYNC POLICY</p>
                                    <SummaryItem title='APPLICATIONS SYNC'>{spec.syncPolicy.applicationsSync || 'sync (default)'}</SummaryItem>
                                    <SummaryItem title='PRESERVE RESOURCES ON DELETION'>{spec.syncPolicy.preserveResourcesOnDeletion ? 'true' : 'false'}</SummaryItem>
                                </div>
                            </div>
                        )}
                    </div>
                )
            },
            {
                title: 'MANIFEST',
                key: 'manifest',
                content: <YamlEditor minHeight={800} input={appSet.spec} hideModeButtons={true} />
            },
            {
                title: 'EVENTS',
                key: 'event',
                content: (
                    <div className='application-resource-events'>
                        <DataLoader
                            load={() => services.applications.appSetEvents(appSet.metadata.name, appSet.metadata.namespace)}
                            loadingRenderer={() => <MockupList height={50} marginTop={10} />}>
                            {events => <EventsList events={events} />}
                        </DataLoader>
                    </div>
                )
            }
        ];

        const extensionTabs = services.extensions.getResourceTabs('argoproj.io', 'ApplicationSet').map((ext, i) => ({
            title: ext.title,
            key: `extension-${i}`,
            content: <ext.component resource={{...appSet, status: appSet.status || {}} as any} tree={{nodes: []} as any} application={appSet as any} />,
            icon: ext.icon
        }));

        return tabs.concat(extensionTabs);
    };

    return (
        <div style={{width: '100%', height: '100%'}}>
            <div className='resource-details__header'>
                <div style={{display: 'flex', flexDirection: 'column', marginRight: '15px', alignItems: 'center', fontSize: '12px'}}>
                    <ResourceIcon group='argoproj.io' kind='ApplicationSet' />
                    {ResourceLabel({kind: 'ApplicationSet'})}
                </div>
                <h1>{appSet.metadata.name}</h1>
                <HealthStatusIcon state={{status: healthStatus, message: ''}} />
            </div>
            <Tabs navTransparent={true} tabs={getTabs()} selectedTabKey={tab} onTabSelected={selected => appContext.navigation.goto('.', {tab: selected}, {replace: true})} />
        </div>
    );
};

const SummaryItem = ({title, children}: {title: string; children: React.ReactNode}) => (
    <div className='row white-box__details-row'>
        <div className='columns small-3'>{title}</div>
        <div className='columns small-9'>{children}</div>
    </div>
);

function getConditionCounts(conditions: models.ApplicationSetCondition[]) {
    const counts = {info: 0, warning: 0, error: 0};
    if (!conditions) {
        return counts;
    }
    conditions.forEach(c => {
        const category = getAppSetConditionCategory(c);
        counts[category]++;
    });
    return counts;
}
