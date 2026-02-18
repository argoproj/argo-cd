import {MockupList, Tab, Tabs} from 'argo-ui';
import * as React from 'react';
import {DataLoader, EventsList, YamlEditor} from '../../../shared/components';
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
    tab?: string;
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
                    <div className='application-summary'>
                        <div className='white-box'>
                            <div className='white-box__details'>
                                <p>{appSet.metadata.name.toLocaleUpperCase()}</p>
                                <SummaryItem title='NAMESPACE'>{appSet.metadata.namespace}</SummaryItem>
                                <SummaryItem title='CREATED AT'>
                                    {appSet.metadata.creationTimestamp && <Timestamp date={appSet.metadata.creationTimestamp} />}
                                </SummaryItem>
                                <SummaryItem title='HEALTH'>
                                    <HealthStatusIcon state={{status: healthStatus, message: ''}} /> {healthStatus}
                                </SummaryItem>
                                {conditions.length > 0 && (
                                    <SummaryItem title='CONDITIONS'>
                                        <a onClick={() => appContext.navigation.goto('.', {conditions: 'true'}, {replace: true})}>
                                            {conditionCounts.error > 0 && <span>{conditionCounts.error} Error{conditionCounts.error !== 1 && 's'}</span>}
                                            {conditionCounts.warning > 0 && <span> {conditionCounts.warning} Warning{conditionCounts.warning !== 1 && 's'}</span>}
                                            {conditionCounts.info > 0 && <span> {conditionCounts.info} Info</span>}
                                            {conditionCounts.error === 0 && conditionCounts.warning === 0 && conditionCounts.info === 0 && <span>{conditions.length} Condition{conditions.length !== 1 && 's'}</span>}
                                        </a>
                                    </SummaryItem>
                                )}
                                <SummaryItem title='LABELS'>
                                    {Object.keys(appSet.metadata.labels || {}).length > 0
                                        ? Object.entries(appSet.metadata.labels).map(([k, v]) => `${k}=${v}`).join(' ')
                                        : ''}
                                </SummaryItem>
                                <SummaryItem title='ANNOTATIONS'>
                                    {Object.keys(appSet.metadata.annotations || {}).length > 0
                                        ? Object.entries(appSet.metadata.annotations).map(([k, v]) => `${k}=${v}`).join(' ')
                                        : ''}
                                </SummaryItem>
                            </div>
                        </div>

                        {spec?.syncPolicy && (
                            <div className='white-box'>
                                <div className='white-box__details'>
                                    <p>SYNC POLICY</p>
                                    <SummaryItem title='APPLICATIONS SYNC'>
                                        {spec.syncPolicy.applicationsSync || 'sync (default)'}
                                    </SummaryItem>
                                    <SummaryItem title='PRESERVE RESOURCES ON DELETION'>
                                        {spec.syncPolicy.preserveResourcesOnDeletion ? 'true' : 'false'}
                                    </SummaryItem>
                                </div>
                            </div>
                        )}

                        {spec?.strategy && (
                            <div className='white-box'>
                                <div className='white-box__details'>
                                    <p>STRATEGY</p>
                                    <SummaryItem title='TYPE'>{spec.strategy.type || 'AllAtOnce'}</SummaryItem>
                                    {spec.strategy.rollingSync?.steps && (
                                        <SummaryItem title='ROLLING SYNC STEPS'>
                                            {spec.strategy.rollingSync.steps.map((step: any, i: number) => (
                                                <div key={i} style={{marginBottom: '4px'}}>
                                                    Step {i + 1}: maxUpdate={step.maxUpdate != null ? String(step.maxUpdate) : 'N/A'}
                                                    {step.matchExpressions?.map((expr: any, j: number) => (
                                                        <div key={j} style={{marginLeft: '16px', fontSize: '12px'}}>
                                                            {expr.key} {expr.operator} [{expr.values?.join(', ')}]
                                                        </div>
                                                    ))}
                                                </div>
                                            ))}
                                        </SummaryItem>
                                    )}
                                </div>
                            </div>
                        )}

                        {spec?.template && (
                            <div className='white-box'>
                                <div className='white-box__details'>
                                    <p>TEMPLATE</p>
                                    <SummaryItem title='PROJECT'>{spec.template.spec?.project || 'default'}</SummaryItem>
                                    {spec.template.spec?.destination && (
                                        <>
                                            <SummaryItem title='DESTINATION SERVER'>
                                                {spec.template.spec.destination.server || spec.template.spec.destination.name || 'N/A'}
                                            </SummaryItem>
                                            <SummaryItem title='DESTINATION NAMESPACE'>
                                                {spec.template.spec.destination.namespace || 'N/A'}
                                            </SummaryItem>
                                        </>
                                    )}
                                    {spec.template.spec?.source && (
                                        <>
                                            <SummaryItem title='REPO URL'>{spec.template.spec.source.repoURL || 'N/A'}</SummaryItem>
                                            <SummaryItem title='TARGET REVISION'>{spec.template.spec.source.targetRevision || 'HEAD'}</SummaryItem>
                                            <SummaryItem title='PATH'>{spec.template.spec.source.path || 'N/A'}</SummaryItem>
                                            {spec.template.spec.source.chart && (
                                                <SummaryItem title='CHART'>{spec.template.spec.source.chart}</SummaryItem>
                                            )}
                                        </>
                                    )}
                                    {spec.template.spec?.sources && spec.template.spec.sources.length > 0 && (
                                        <SummaryItem title='SOURCES'>
                                            {spec.template.spec.sources.map((src: any, i: number) => (
                                                <div key={i} style={{marginBottom: '8px'}}>
                                                    <div><strong>Source {i + 1}:</strong> {src.repoURL}</div>
                                                    {src.targetRevision && <div style={{marginLeft: '16px'}}>Revision: {src.targetRevision}</div>}
                                                    {src.path && <div style={{marginLeft: '16px'}}>Path: {src.path}</div>}
                                                    {src.chart && <div style={{marginLeft: '16px'}}>Chart: {src.chart}</div>}
                                                </div>
                                            ))}
                                        </SummaryItem>
                                    )}
                                </div>
                            </div>
                        )}
                    </div>
                )
            },
            {
                title: 'MANIFEST',
                key: 'manifest',
                content: (
                    <YamlEditor
                        minHeight={800}
                        input={appSet.spec}
                        hideModeButtons={true}
                    />
                )
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
            <Tabs
                navTransparent={true}
                tabs={getTabs()}
                selectedTabKey={tab}
                onTabSelected={selected => appContext.navigation.goto('.', {tab: selected}, {replace: true})}
            />
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
