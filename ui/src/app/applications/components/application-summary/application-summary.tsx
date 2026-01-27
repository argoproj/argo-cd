/* eslint-disable no-prototype-builtins */
import {AutocompleteField, Checkbox, DropDownMenu, ErrorNotification, FormField, FormSelect, HelpIcon, NotificationType} from 'argo-ui';
import * as React from 'react';
import {FormApi, Text} from 'react-form';
import {
    ClipboardText,
    Cluster,
    DataLoader,
    EditablePanel,
    EditablePanelItem,
    EditablePanelContent,
    Expandable,
    MapInputField,
    NumberField,
    Repo,
    Revision,
    RevisionHelpIcon
} from '../../../shared/components';
import {BadgePanel} from '../../../shared/components';
import {AuthSettingsCtx, Consumer, ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

import {ApplicationSyncOptionsField} from '../application-sync-options/application-sync-options';
import {RevisionFormField} from '../revision-form-field/revision-form-field';
import {ComparisonStatusIcon, HealthStatusIcon, syncStatusMessage, urlPattern, formatCreationTimestamp, getAppDefaultSource, getAppSpecDefaultSource} from '../utils';
import {ApplicationRetryOptions} from '../application-retry-options/application-retry-options';
import {ApplicationRetryView} from '../application-retry-view/application-retry-view';
import {Link} from 'react-router-dom';
import {EditNotificationSubscriptions, useEditNotificationSubscriptions} from './edit-notification-subscriptions';
import {EditAnnotations} from './edit-annotations';

import './application-summary.scss';
import {DeepLinks} from '../../../shared/components/deep-links';

function swap(array: any[], a: number, b: number) {
    array = array.slice();
    [array[a], array[b]] = [array[b], array[a]];
    return array;
}

function processPath(path: string) {
    if (path !== null && path !== undefined) {
        if (path === '.') {
            return '(root)';
        }
        return path;
    }
    return '';
}

export interface ApplicationSummaryProps {
    app: models.Application;
    updateApp: (app: models.Application, query: {validate?: boolean}) => Promise<any>;
}

export const ApplicationSummary = (props: ApplicationSummaryProps) => {
    const app = JSON.parse(JSON.stringify(props.app)) as models.Application;
    const source = getAppDefaultSource(app);
    const isHelm = source.hasOwnProperty('chart');
    const initialState = app.spec.destination.server === undefined ? 'NAME' : 'URL';
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const [destFormat, setDestFormat] = React.useState(initialState);
    const [, setChangeSync] = React.useState(false);
    const [isHydratorEnabled, setIsHydratorEnabled] = React.useState(!!app.spec.sourceHydrator);
    const [savedSyncSource, setSavedSyncSource] = React.useState(app.spec.sourceHydrator?.syncSource || {targetBranch: '', path: ''});

    const notificationSubscriptions = useEditNotificationSubscriptions(app.metadata.annotations || {});
    const updateApp = notificationSubscriptions.withNotificationSubscriptions(props.updateApp);

    const updateAppSource = async (input: models.Application, query: {validate?: boolean}) => {
        const hydrateTo = input.spec.sourceHydrator?.hydrateTo;
        if (hydrateTo && !hydrateTo.targetBranch?.trim() && input.spec.sourceHydrator) {
            delete input.spec.sourceHydrator.hydrateTo;
        }
        return updateApp(input, query);
    };

    const hasMultipleSources = app.spec.sources && app.spec.sources.length > 0;
    const isHydrator = isHydratorEnabled;
    const repoType = source.repoURL.startsWith('oci://') ? 'oci' : (source.hasOwnProperty('chart') && 'helm') || 'git';

    const attributes = [
        {
            title: 'PROJECT',
            view: <Link to={'/settings/projects/' + app.spec.project}>{app.spec.project}</Link>,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.projects.list('items.metadata.name').then(projs => projs.map(item => item.metadata.name))}>
                    {projects => <FormField formApi={formApi} field='spec.project' component={FormSelect} componentProps={{options: projects}} />}
                </DataLoader>
            )
        },
        {
            title: 'LABELS',
            view: Object.keys(app.metadata.labels || {})
                .map(label => `${label}=${app.metadata.labels[label]}`)
                .join(' '),
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='metadata.labels' component={MapInputField} />
        },
        {
            title: 'ANNOTATIONS',
            view: (
                <Expandable height={48}>
                    {Object.keys(app.metadata.annotations || {})
                        .map(annotation => `${annotation}=${app.metadata.annotations[annotation]}`)
                        .join(' ')}
                </Expandable>
            ),
            edit: (formApi: FormApi) => <EditAnnotations formApi={formApi} app={app} />
        },
        {
            title: 'NOTIFICATION SUBSCRIPTIONS',
            view: false, // eventually the subscription input values will be merged in 'ANNOTATIONS', therefore 'ANNOATIONS' section is responsible to represent subscription values,
            edit: () => <EditNotificationSubscriptions {...notificationSubscriptions} />
        },
        {
            title: 'CLUSTER',
            view: <Cluster server={app.spec.destination.server} name={app.spec.destination.name} showUrl={true} />,
            edit: (formApi: FormApi) => (
                <DataLoader load={() => services.clusters.list().then(clusters => clusters.sort())}>
                    {clusters => {
                        return (
                            <div className='row'>
                                {(destFormat.toUpperCase() === 'URL' && (
                                    <div className='columns small-10'>
                                        <FormField
                                            formApi={formApi}
                                            field='spec.destination.server'
                                            componentProps={{items: clusters.map(cluster => cluster.server)}}
                                            component={AutocompleteField}
                                        />
                                    </div>
                                )) || (
                                    <div className='columns small-10'>
                                        <FormField
                                            formApi={formApi}
                                            field='spec.destination.name'
                                            componentProps={{items: clusters.map(cluster => cluster.name)}}
                                            component={AutocompleteField}
                                        />
                                    </div>
                                )}
                                <div className='columns small-2'>
                                    <div>
                                        <DropDownMenu
                                            anchor={() => (
                                                <p>
                                                    {destFormat.toUpperCase()} <i className='fa fa-caret-down' />
                                                </p>
                                            )}
                                            items={['URL', 'NAME'].map((type: 'URL' | 'NAME') => ({
                                                title: type,
                                                action: () => {
                                                    if (destFormat !== type) {
                                                        const updatedApp = formApi.getFormState().values as models.Application;
                                                        if (type === 'URL') {
                                                            updatedApp.spec.destination.server = '';
                                                            delete updatedApp.spec.destination.name;
                                                        } else {
                                                            updatedApp.spec.destination.name = '';
                                                            delete updatedApp.spec.destination.server;
                                                        }
                                                        formApi.setAllValues(updatedApp);
                                                        setDestFormat(type);
                                                    }
                                                }
                                            }))}
                                        />
                                    </div>
                                </div>
                            </div>
                        );
                    }}
                </DataLoader>
            )
        },
        {
            title: 'NAMESPACE',
            view: <ClipboardText text={app.spec.destination.namespace} />,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.destination.namespace' component={Text} />
        },
        {
            title: 'CREATED AT',
            view: formatCreationTimestamp(app.metadata.creationTimestamp)
        },
        {
            title: 'REVISION HISTORY LIMIT',
            view: app.spec.revisionHistoryLimit,
            edit: (formApi: FormApi) => (
                <div style={{position: 'relative'}}>
                    <FormField
                        formApi={formApi}
                        field='spec.revisionHistoryLimit'
                        componentProps={{style: {paddingRight: '1em', right: '1em'}, placeholder: '10'}}
                        component={NumberField}
                    />
                    <div style={{position: 'absolute', right: '0', top: '0'}}>
                        <HelpIcon
                            title='This limits the number of items kept in the apps revision history.
    This should only be changed in exceptional circumstances.
    Setting to zero will store no history. This will reduce storage used.
    Increasing will increase the space used to store the history, so we do not recommend increasing it.
    Default is 10.'
                        />
                    </div>
                </div>
            )
        },
        {
            title: 'SYNC OPTIONS',
            view: (
                <div style={{display: 'flex', flexWrap: 'wrap'}}>
                    {((app.spec.syncPolicy || {}).syncOptions || []).map(opt =>
                        opt.endsWith('=true') || opt.endsWith('=false') ? (
                            <div key={opt} style={{marginRight: '10px'}}>
                                <i className={`fa fa-${opt.includes('=true') ? 'check-square' : 'times'}`} /> {opt.replace('=true', '').replace('=false', '')}
                            </div>
                        ) : (
                            <div key={opt} style={{marginRight: '10px'}}>
                                {opt}
                            </div>
                        )
                    )}
                </div>
            ),
            edit: (formApi: FormApi) => (
                <div>
                    <FormField formApi={formApi} field='spec.syncPolicy.syncOptions' component={ApplicationSyncOptionsField} />
                </div>
            )
        },
        {
            title: 'RETRY OPTIONS',
            view: <ApplicationRetryView initValues={app.spec.syncPolicy ? app.spec.syncPolicy.retry : null} />,
            edit: (formApi: FormApi) => (
                <div>
                    <ApplicationRetryOptions formApi={formApi} initValues={app.spec.syncPolicy ? app.spec.syncPolicy.retry : null} field='spec.syncPolicy.retry' />
                </div>
            )
        },
        {
            title: 'STATUS',
            view: (
                <span>
                    <ComparisonStatusIcon status={app.status.sync.status} /> {app.status.sync.status} {syncStatusMessage(app)}
                </span>
            )
        },
        {
            title: 'HEALTH',
            view: (
                <span>
                    <HealthStatusIcon state={app.status.health} /> {app.status.health.status}
                </span>
            )
        },
        {
            title: 'LINKS',
            view: (
                <DataLoader load={() => services.applications.getLinks(app.metadata.name, app.metadata.namespace)} input={app} key='appLinks'>
                    {(links: models.LinksResponse) => <DeepLinks links={links.items} />}
                </DataLoader>
            )
        }
    ];

    const urls = app.status.summary.externalURLs || [];
    if (urls.length > 0) {
        attributes.push({
            title: 'URLs',
            view: (
                <div className='application-summary__links-rows'>
                    {urls
                        .map(item => item.split('|'))
                        .map((parts, i) => (
                            <div className='application-summary__links-row'>
                                <a key={i} href={parts.length > 1 ? parts[1] : parts[0]} target='_blank'>
                                    {parts[0]} &nbsp;
                                </a>
                            </div>
                        ))}
                </div>
            )
        });
    }

    if ((app.status.summary.images || []).length) {
        attributes.push({
            title: 'IMAGES',
            view: (
                <div className='application-summary__labels'>
                    {(app.status.summary.images || []).sort().map(image => (
                        <span className='application-summary__label' key={image}>
                            {image}
                        </span>
                    ))}
                </div>
            )
        });
    }

    const standardSourceItems: EditablePanelItem[] = useAuthSettingsCtx?.hydratorEnabled
        ? [
              {
                  title: 'ENABLE HYDRATOR',
                  view: false, // Hide in view mode
                  edit: (formApi: FormApi) => (
                      <div className='checkbox-container'>
                          <Checkbox
                              onChange={(val: boolean) => {
                                  const updatedApp = formApi.getFormState().values as models.Application;
                                  if (val) {
                                      // Enable hydrator - move source to sourceHydrator.drySource
                                      if (!updatedApp.spec.sourceHydrator) {
                                          updatedApp.spec.sourceHydrator = {
                                              drySource: {
                                                  repoURL: updatedApp.spec.source.repoURL,
                                                  targetRevision: updatedApp.spec.source.targetRevision,
                                                  path: updatedApp.spec.source.path
                                              },
                                              syncSource: savedSyncSource
                                          };
                                          delete updatedApp.spec.source;
                                      }
                                  } else {
                                      // Disable hydrator - save sync source values and move drySource back to source
                                      if (updatedApp.spec.sourceHydrator) {
                                          setSavedSyncSource(updatedApp.spec.sourceHydrator.syncSource);
                                          updatedApp.spec.source = updatedApp.spec.sourceHydrator.drySource;
                                          delete updatedApp.spec.sourceHydrator;
                                      }
                                  }
                                  formApi.setAllValues(updatedApp);
                                  setIsHydratorEnabled(val);
                              }}
                              checked={!!(formApi.getFormState().values as models.Application).spec.sourceHydrator}
                              id='enable-hydrator'
                          />
                          <label htmlFor='enable-hydrator'>SOURCE HYDRATOR ENABLED</label>
                          <HelpIcon title='Enable source hydrator to sync rendered manifests to a separate Git repository' />
                      </div>
                  )
              }
          ]
        : [];

    const sourceItems: EditablePanelItem[] = [
        {
            title: 'REPO URL',
            view: <Repo url={app.spec.source?.repoURL} />,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.repoURL' component={Text} />
        },
        ...(isHelm
            ? [
                  {
                      title: 'CHART',
                      view: <span>{source && `${source.chart}:${source.targetRevision}`}</span>,
                      edit: (formApi: FormApi) => (
                          <DataLoader
                              input={{repoURL: getAppSpecDefaultSource(formApi.getFormState().values.spec).repoURL}}
                              load={src => services.repos.charts(src.repoURL).catch(() => new Array<models.HelmChart>())}>
                              {(charts: models.HelmChart[]) => (
                                  <div className='row'>
                                      <div className='columns small-8'>
                                          <FormField
                                              formApi={formApi}
                                              field='spec.source.chart'
                                              component={AutocompleteField}
                                              componentProps={{items: charts.map(chart => chart.name), filterSuggestions: true}}
                                          />
                                      </div>
                                      <DataLoader
                                          input={{charts, chart: getAppSpecDefaultSource(formApi.getFormState().values.spec).chart}}
                                          load={async data => {
                                              const chartInfo = data.charts.find(chart => chart.name === data.chart);
                                              return (chartInfo && chartInfo.versions) || new Array<string>();
                                          }}>
                                          {(versions: string[]) => (
                                              <div className='columns small-4'>
                                                  <FormField
                                                      formApi={formApi}
                                                      field='spec.source.targetRevision'
                                                      component={AutocompleteField}
                                                      componentProps={{items: versions}}
                                                  />
                                                  <RevisionHelpIcon type='helm' top='0' />
                                              </div>
                                          )}
                                      </DataLoader>
                                  </div>
                              )}
                          </DataLoader>
                      )
                  }
              ]
            : [
                  {
                      title: 'TARGET REVISION',
                      view: <Revision repoUrl={source.repoURL} revision={source.targetRevision || 'HEAD'} />,
                      edit: (formApi: FormApi) => (
                          <RevisionFormField
                              helpIconTop={'0'}
                              hideLabel={true}
                              formApi={formApi}
                              fieldValue='spec.source.targetRevision'
                              repoURL={source.repoURL}
                              repoType={repoType}
                          />
                      )
                  },
                  {
                      title: 'PATH',
                      view: (
                          <Revision repoUrl={source.repoURL} revision={source.targetRevision || 'HEAD'} path={source.path} isForPath={true}>
                              {processPath(source.path)}
                          </Revision>
                      ),
                      edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.source.path' component={Text} />
                  }
              ])
    ];

    const drySourceItems: EditablePanelItem[] = [
        {
            title: 'REPO URL',
            view: <Repo url={app.spec.sourceHydrator?.drySource?.repoURL} />,
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.sourceHydrator.drySource.repoURL' component={Text} />
        },
        ...(isHelm
            ? [
                  {
                      title: 'CHART',
                      view: <span>{source && `${source.chart}:${source.targetRevision}`}</span>,
                      edit: (formApi: FormApi) => (
                          <DataLoader
                              input={{repoURL: getAppSpecDefaultSource(formApi.getFormState().values.spec).repoURL}}
                              load={src => services.repos.charts(src.repoURL).catch(() => new Array<models.HelmChart>())}>
                              {(charts: models.HelmChart[]) => (
                                  <div className='row'>
                                      <div className='columns small-8'>
                                          <FormField
                                              formApi={formApi}
                                              field='spec.sourceHydrator.drySource.chart'
                                              component={AutocompleteField}
                                              componentProps={{items: charts.map(chart => chart.name), filterSuggestions: true}}
                                          />
                                      </div>
                                      <DataLoader
                                          input={{charts, chart: getAppSpecDefaultSource(formApi.getFormState().values.spec).chart}}
                                          load={async data => {
                                              const chartInfo = data.charts.find(chart => chart.name === data.chart);
                                              return (chartInfo && chartInfo.versions) || new Array<string>();
                                          }}>
                                          {(versions: string[]) => (
                                              <div className='columns small-4'>
                                                  <FormField
                                                      formApi={formApi}
                                                      field='spec.sourceHydrator.drySource.targetRevision'
                                                      component={AutocompleteField}
                                                      componentProps={{items: versions}}
                                                  />
                                                  <RevisionHelpIcon type='helm' top='0' />
                                              </div>
                                          )}
                                      </DataLoader>
                                  </div>
                              )}
                          </DataLoader>
                      )
                  }
              ]
            : [
                  {
                      title: 'TARGET REVISION',
                      view: <Revision repoUrl={app.spec.sourceHydrator?.drySource?.repoURL} revision={app.spec.sourceHydrator?.drySource?.targetRevision || 'HEAD'} />,
                      edit: (formApi: FormApi) => (
                          <RevisionFormField
                              helpIconTop={'0'}
                              hideLabel={true}
                              formApi={formApi}
                              fieldValue='spec.sourceHydrator.drySource.targetRevision'
                              repoURL={app.spec.sourceHydrator?.drySource?.repoURL}
                              repoType={repoType}
                          />
                      )
                  },
                  {
                      title: 'PATH',
                      view: (
                          <Revision
                              repoUrl={app.spec.sourceHydrator?.drySource?.repoURL}
                              revision={app.spec.sourceHydrator?.drySource?.targetRevision || 'HEAD'}
                              path={app.spec.sourceHydrator?.drySource?.path}
                              isForPath={true}>
                              {processPath(app.spec.sourceHydrator?.drySource?.path)}
                          </Revision>
                      ),
                      edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.sourceHydrator.drySource.path' component={Text} />
                  }
              ])
    ];

    const syncSourceItems: EditablePanelItem[] = [
        {
            title: 'TARGET BRANCH',
            view: <Revision repoUrl={app.spec.sourceHydrator?.drySource?.repoURL} revision={app.spec.sourceHydrator?.syncSource?.targetBranch || 'HEAD'} />,
            edit: (formApi: FormApi) => (
                <RevisionFormField
                    helpIconTop={'0'}
                    hideLabel={true}
                    formApi={formApi}
                    fieldValue='spec.sourceHydrator.syncSource.targetBranch'
                    repoURL={source.repoURL}
                    repoType='git'
                    revisionType='Branches'
                />
            )
        },
        {
            title: 'PATH',
            view: (
                <Revision
                    repoUrl={app.spec.sourceHydrator?.drySource?.repoURL}
                    revision={app.spec.sourceHydrator?.syncSource?.targetBranch || 'HEAD'}
                    path={app.spec.sourceHydrator?.syncSource?.path}
                    isForPath={true}>
                    {processPath(app.spec.sourceHydrator?.syncSource?.path)}
                </Revision>
            ),
            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.sourceHydrator.syncSource.path' component={Text} />
        }
    ];

    const sourceAttributes: (EditablePanelItem | EditablePanelContent)[] = isHydrator
        ? [
              ...standardSourceItems,
              {
                  sectionName: 'Dry Source',
                  items: drySourceItems
              },
              {
                  sectionName: 'Sync Source',
                  items: syncSourceItems
              }
          ]
        : [...standardSourceItems, ...sourceItems];

    async function setAutoSync(ctx: ContextApis, confirmationTitle: string, confirmationText: string, prune: boolean, selfHeal: boolean, enable: boolean) {
        const confirmed = await ctx.popup.confirm(confirmationTitle, confirmationText);
        if (confirmed) {
            try {
                setChangeSync(true);
                const updatedApp = JSON.parse(JSON.stringify(props.app)) as models.Application;
                if (!updatedApp.spec.syncPolicy) {
                    updatedApp.spec.syncPolicy = {};
                }

                updatedApp.spec.syncPolicy.automated = {prune, selfHeal, enabled: enable};
                await updateApp(updatedApp, {validate: false});
            } catch (e) {
                ctx.notifications.show({
                    content: <ErrorNotification title={`Unable to "${confirmationTitle.replace(/\?/g, '')}:`} e={e} />,
                    type: NotificationType.Error
                });
            } finally {
                setChangeSync(false);
            }
        }
    }

    const items = app.spec.info || [];
    const [adjustedCount, setAdjustedCount] = React.useState(0);

    const added = new Array<{name: string; value: string; key: string}>();
    for (let i = 0; i < adjustedCount; i++) {
        added.push({name: '', value: '', key: (items.length + i).toString()});
    }
    for (let i = 0; i > adjustedCount; i--) {
        items.pop();
    }
    const allItems = items.concat(added);
    const infoItems: EditablePanelItem[] = allItems
        .map((info, i) => ({
            key: i.toString(),
            title: info.name,
            view: info.value.match(urlPattern) ? (
                <a href={info.value} target='__blank'>
                    {info.value}
                </a>
            ) : (
                info.value
            ),
            titleEdit: (formApi: FormApi) => (
                <React.Fragment>
                    {i > 0 && (
                        <i
                            className='fa fa-sort-up application-summary__sort-icon'
                            onClick={() => {
                                formApi.setValue('spec.info', swap(formApi.getFormState().values.spec.info || [], i, i - 1));
                            }}
                        />
                    )}
                    <FormField formApi={formApi} field={`spec.info[${[i]}].name`} component={Text} componentProps={{style: {width: '99%'}}} />
                    {i < allItems.length - 1 && (
                        <i
                            className='fa fa-sort-down application-summary__sort-icon'
                            onClick={() => {
                                formApi.setValue('spec.info', swap(formApi.getFormState().values.spec.info || [], i, i + 1));
                            }}
                        />
                    )}
                </React.Fragment>
            ),
            edit: (formApi: FormApi) => (
                <React.Fragment>
                    <FormField formApi={formApi} field={`spec.info[${[i]}].value`} component={Text} />
                    <i
                        className='fa fa-times application-summary__remove-icon'
                        onClick={() => {
                            const values = (formApi.getFormState().values.spec.info || []) as Array<any>;
                            formApi.setValue('spec.info', [...values.slice(0, i), ...values.slice(i + 1, values.length)]);
                            setAdjustedCount(adjustedCount - 1);
                        }}
                    />
                </React.Fragment>
            )
        }))
        .concat({
            key: '-1',
            title: '',
            titleEdit: () => (
                <button
                    className='argo-button argo-button--base'
                    onClick={() => {
                        setAdjustedCount(adjustedCount + 1);
                    }}>
                    ADD NEW ITEM
                </button>
            ),
            view: null as any,
            edit: null
        });

    return (
        <div className='application-summary'>
            <EditablePanel
                save={updateApp}
                view={hasMultipleSources ? <>This is a multi-source app, see the Sources tab for repository URLs and source-related information.</> : null}
                validate={input => ({
                    'spec.project': !input.spec.project && 'Project name is required',
                    'spec.destination.server': !input.spec.destination.server && input.spec.destination.hasOwnProperty('server') && 'Cluster server is required',
                    'spec.destination.name': !input.spec.destination.name && input.spec.destination.hasOwnProperty('name') && 'Cluster name is required'
                })}
                values={app}
                title={app.metadata.name.toLocaleUpperCase()}
                items={attributes}
                onModeSwitch={() => notificationSubscriptions.onResetNotificationSubscriptions()}
            />
            {!hasMultipleSources && (
                <EditablePanel
                    save={updateApp}
                    values={app}
                    title='SOURCE'
                    items={sourceAttributes}
                    hasMultipleSources={hasMultipleSources}
                    onModeSwitch={() => notificationSubscriptions.onResetNotificationSubscriptions()}
                />
            )}
            <Consumer>
                {ctx => (
                    <div className='white-box'>
                        <div className='white-box__details'>
                            <p>SYNC POLICY</p>
                            <div className='row white-box__details-row'>
                                <div className='columns small-3'>
                                    {app.spec.syncPolicy?.automated && app.spec.syncPolicy.automated.enabled !== false ? <span>AUTOMATED</span> : <span>NONE</span>}
                                </div>
                            </div>
                            <div className='row white-box__details-row'>
                                <div className='columns small-12'>
                                    <div className='checkbox-container'>
                                        <Checkbox
                                            onChange={async (val: boolean) => {
                                                const automated = app.spec.syncPolicy?.automated || {prune: false, selfHeal: false};
                                                setAutoSync(
                                                    ctx,
                                                    val ? 'Enable Auto-Sync?' : 'Disable Auto-Sync?',
                                                    val
                                                        ? 'If checked, application will automatically sync when changes are detected'
                                                        : 'Are you sure you want to disable automated application synchronization',
                                                    automated.prune,
                                                    automated.selfHeal,
                                                    val
                                                );
                                            }}
                                            checked={app.spec.syncPolicy?.automated && app.spec.syncPolicy.automated.enabled !== false}
                                            id='enable-auto-sync'
                                        />
                                        <label htmlFor='enable-auto-sync'>ENABLE AUTO-SYNC</label>
                                        <HelpIcon title='If checked, application will automatically sync when changes are detected' />
                                    </div>
                                </div>
                            </div>
                            {app.spec.syncPolicy && app.spec.syncPolicy.automated && (
                                <React.Fragment>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-12'>
                                            <div className='checkbox-container'>
                                                <Checkbox
                                                    onChange={async (prune: boolean) => {
                                                        const automated = app.spec.syncPolicy?.automated || {selfHeal: false, enabled: false};
                                                        setAutoSync(
                                                            ctx,
                                                            prune ? 'Enable Prune Resources?' : 'Disable Prune Resources?',
                                                            prune
                                                                ? 'Are you sure you want to enable resource pruning during automated application synchronization?'
                                                                : 'Are you sure you want to disable resource pruning during automated application synchronization?',
                                                            prune,
                                                            automated.selfHeal,
                                                            automated.enabled
                                                        );
                                                    }}
                                                    checked={!!app.spec.syncPolicy?.automated?.prune}
                                                    id='prune-resources'
                                                />
                                                <label htmlFor='prune-resources'>PRUNE RESOURCES</label>
                                            </div>
                                        </div>
                                    </div>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-12'>
                                            <div className='checkbox-container'>
                                                <Checkbox
                                                    onChange={async (selfHeal: boolean) => {
                                                        const automated = app.spec.syncPolicy?.automated || {prune: false, enabled: false};
                                                        setAutoSync(
                                                            ctx,
                                                            selfHeal ? 'Enable Self Heal?' : 'Disable Self Heal?',
                                                            selfHeal
                                                                ? 'If checked, application will automatically sync when changes are detected'
                                                                : 'Are you sure you want to enable automated self healing?',
                                                            automated.prune,
                                                            selfHeal,
                                                            automated.enabled
                                                        );
                                                    }}
                                                    checked={!!app.spec.syncPolicy?.automated?.selfHeal}
                                                    id='self-heal'
                                                />
                                                <label htmlFor='self-heal'>SELF HEAL</label>
                                            </div>
                                        </div>
                                    </div>
                                </React.Fragment>
                            )}
                        </div>
                    </div>
                )}
            </Consumer>
            <BadgePanel app={props.app.metadata.name} appNamespace={props.app.metadata.namespace} nsEnabled={useAuthSettingsCtx?.appsInAnyNamespaceEnabled} />
            <EditablePanel
                save={updateApp}
                values={app}
                title='INFO'
                items={infoItems}
                onModeSwitch={() => {
                    setAdjustedCount(0);
                    notificationSubscriptions.onResetNotificationSubscriptions();
                }}
            />
        </div>
    );
};
