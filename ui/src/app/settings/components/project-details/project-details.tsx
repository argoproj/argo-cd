import {AutocompleteField, FormField, HelpIcon, NotificationsApi, NotificationType, SlidingPanel, Tabs, Tooltip} from 'argo-ui';
import classNames from 'classnames';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {FormApi, Text} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {BadgePanel, CheckboxField, DataLoader, EditablePanel, ErrorNotification, MapInputField, Page, Query} from '../../../shared/components';
import {AppContext, Consumer, AuthSettingsCtx} from '../../../shared/context';
import {GroupKind, Groups, Project, DetailedProjectsResponse, ProjectSpec, ResourceKinds} from '../../../shared/models';
import {CreateJWTTokenParams, DeleteJWTTokenParams, ProjectRoleParams, services} from '../../../shared/services';

import {SyncWindowStatusIcon} from '../../../applications/components/utils';
import {ProjectSyncWindowsParams} from '../../../shared/services/projects-service';
import {ProjectEvents} from '../project-events/project-events';
import {ProjectRoleEditPanel} from '../project-role-edit-panel/project-role-edit-panel';
import {ProjectSyncWindowsEditPanel} from '../project-sync-windows-edit-panel/project-sync-windows-edit-panel';
import {ResourceListsPanel} from './resource-lists-panel';
import {DeepLinks} from '../../../shared/components/deep-links';

require('./project-details.scss');

interface ProjectDetailsState {
    token: string;
}

function removeEl(items: any[], index: number) {
    return items.slice(0, index).concat(items.slice(index + 1));
}

function helpTip(text: string) {
    return (
        <Tooltip content={text}>
            <span style={{fontSize: 'smaller'}}>
                {' '}
                <i className='fas fa-info-circle' />
            </span>
        </Tooltip>
    );
}

function emptyMessage(title: string) {
    return <p>Project has no {title}</p>;
}

function reduceGlobal(projs: Project[]): ProjectSpec & {count: number} {
    return (projs || []).reduce(
        (merged, proj) => {
            merged.clusterResourceBlacklist = merged.clusterResourceBlacklist.concat(proj.spec.clusterResourceBlacklist || []);
            merged.clusterResourceWhitelist = merged.clusterResourceWhitelist.concat(proj.spec.clusterResourceWhitelist || []);
            merged.namespaceResourceBlacklist = merged.namespaceResourceBlacklist.concat(proj.spec.namespaceResourceBlacklist || []);
            merged.namespaceResourceWhitelist = merged.namespaceResourceWhitelist.concat(proj.spec.namespaceResourceWhitelist || []);
            merged.sourceRepos = merged.sourceRepos.concat(proj.spec.sourceRepos || []);
            merged.destinations = merged.destinations.concat(proj.spec.destinations || []);
            merged.sourceNamespaces = merged.sourceNamespaces.concat(proj.spec.sourceNamespaces || []);

            merged.sourceRepos = merged.sourceRepos.filter((item, index) => {
                return (
                    index ===
                    merged.sourceRepos.findIndex(obj => {
                        return obj === item;
                    })
                );
            });

            merged.destinationServiceAccounts = merged.destinationServiceAccounts.filter((item, index) => {
                return (
                    index ===
                    merged.destinationServiceAccounts.findIndex(obj => {
                        return obj.server === item.server && obj.namespace === item.namespace && obj.defaultServiceAccount === item.defaultServiceAccount;
                    })
                );
            });

            merged.destinations = merged.destinations.filter((item, index) => {
                return (
                    index ===
                    merged.destinations.findIndex(obj => {
                        return obj.server === item.server && obj.namespace === item.namespace;
                    })
                );
            });

            merged.clusterResourceBlacklist = merged.clusterResourceBlacklist.filter((item, index) => {
                return (
                    index ===
                    merged.clusterResourceBlacklist.findIndex(obj => {
                        return obj.kind === item.kind && obj.group === item.group;
                    })
                );
            });

            merged.clusterResourceWhitelist = merged.clusterResourceWhitelist.filter((item, index) => {
                return (
                    index ===
                    merged.clusterResourceWhitelist.findIndex(obj => {
                        return obj.kind === item.kind && obj.group === item.group;
                    })
                );
            });

            merged.namespaceResourceBlacklist = merged.namespaceResourceBlacklist.filter((item, index) => {
                return (
                    index ===
                    merged.namespaceResourceBlacklist.findIndex(obj => {
                        return obj.kind === item.kind && obj.group === item.group;
                    })
                );
            });

            merged.namespaceResourceWhitelist = merged.namespaceResourceWhitelist.filter((item, index) => {
                return (
                    index ===
                    merged.namespaceResourceWhitelist.findIndex(obj => {
                        return obj.kind === item.kind && obj.group === item.group;
                    })
                );
            });

            merged.sourceNamespaces = merged.sourceNamespaces.filter((item, index) => {
                return (
                    index ===
                    merged.sourceNamespaces.findIndex(obj => {
                        return obj === item;
                    })
                );
            });
            merged.count += 1;

            return merged;
        },
        {
            clusterResourceBlacklist: new Array<GroupKind>(),
            namespaceResourceBlacklist: new Array<GroupKind>(),
            namespaceResourceWhitelist: new Array<GroupKind>(),
            clusterResourceWhitelist: new Array<GroupKind>(),
            sourceRepos: [],
            sourceNamespaces: [],
            signatureKeys: [],
            destinations: [],
            description: '',
            destinationServiceAccounts: [],
            roles: [],
            count: 0
        }
    );
}

export class ProjectDetails extends React.Component<RouteComponentProps<{name: string}>, ProjectDetailsState> {
    public static contextTypes = {
        apis: PropTypes.object
    };
    private projectRoleFormApi: FormApi;
    private projectSyncWindowsFormApi: FormApi;
    private loader: DataLoader;

    constructor(props: RouteComponentProps<{name: string}>) {
        super(props);
        this.state = {token: ''};
    }

    public render() {
        return (
            <Consumer>
                {ctx => (
                    <Page
                        title='Projects'
                        toolbar={{
                            breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Projects', path: '/settings/projects'}, {title: this.props.match.params.name}],
                            actionMenu: {
                                items: [
                                    {title: 'Add Role', iconClassName: 'fa fa-plus', action: () => ctx.navigation.goto('.', {newRole: true}, {replace: true})},
                                    {title: 'Add Sync Window', iconClassName: 'fa fa-plus', action: () => ctx.navigation.goto('.', {newWindow: true}, {replace: true})},
                                    {
                                        title: 'Delete',
                                        iconClassName: 'fa fa-times-circle',
                                        action: async () => {
                                            const confirmed = await ctx.popup.confirm('Delete project', 'Are you sure you want to delete project?');
                                            if (confirmed) {
                                                try {
                                                    await services.projects.delete(this.props.match.params.name);
                                                    ctx.navigation.goto('/settings/projects', {replace: true});
                                                } catch (e) {
                                                    ctx.notifications.show({
                                                        content: <ErrorNotification title='Unable to delete project' e={e} />,
                                                        type: NotificationType.Error
                                                    });
                                                }
                                            }
                                        }
                                    }
                                ]
                            }
                        }}>
                        <DataLoader
                            load={() => {
                                return services.projects.getDetailed(this.props.match.params.name);
                            }}
                            ref={loader => (this.loader = loader)}>
                            {scopedProj => (
                                <Query>
                                    {params => {
                                        const {project: proj, globalProjects: globalProj} = scopedProj;
                                        return (
                                            <div className='project-details'>
                                                <Tabs
                                                    selectedTabKey={params.get('tab') || 'summary'}
                                                    onTabSelected={tab => ctx.navigation.goto('.', {tab}, {replace: true})}
                                                    navCenter={true}
                                                    tabs={[
                                                        {
                                                            key: 'summary',
                                                            title: 'Summary',
                                                            content: this.summaryTab(proj, reduceGlobal(globalProj), scopedProj)
                                                        },
                                                        {
                                                            key: 'roles',
                                                            title: 'Roles',
                                                            content: this.rolesTab(proj, ctx)
                                                        },
                                                        {
                                                            key: 'windows',
                                                            title: 'Sync Windows',
                                                            content: this.SyncWindowsTab(proj, ctx)
                                                        },
                                                        {
                                                            key: 'events',
                                                            title: 'Events',
                                                            content: this.eventsTab(proj)
                                                        }
                                                    ].map(tab => ({...tab, isOnlyContentScrollable: true, extraVerticalScrollPadding: 160}))}
                                                />
                                                <SlidingPanel
                                                    isMiddle={true}
                                                    isShown={params.get('editRole') !== null || params.get('newRole') !== null}
                                                    onClose={() => {
                                                        this.setState({token: ''});
                                                        ctx.navigation.goto('.', {editRole: null, newRole: null}, {replace: true});
                                                    }}
                                                    header={
                                                        <div>
                                                            <button onClick={() => this.projectRoleFormApi.submitForm(null)} className='argo-button argo-button--base'>
                                                                {params.get('newRole') != null ? 'Create' : 'Update'}
                                                            </button>{' '}
                                                            <button
                                                                onClick={() => {
                                                                    this.setState({token: ''});
                                                                    ctx.navigation.goto('.', {editRole: null, newRole: null}, {replace: true});
                                                                }}
                                                                className='argo-button argo-button--base-o'>
                                                                Cancel
                                                            </button>{' '}
                                                            {params.get('newRole') === null ? (
                                                                <button
                                                                    onClick={async () => {
                                                                        const confirmed = await ctx.popup.confirm(
                                                                            'Delete project role',
                                                                            'Are you sure you want to delete project role?'
                                                                        );
                                                                        if (confirmed) {
                                                                            try {
                                                                                this.projectRoleFormApi.setValue('deleteRole', true);
                                                                                this.projectRoleFormApi.submitForm(null);
                                                                                ctx.navigation.goto('.', {editRole: null}, {replace: true});
                                                                            } catch (e) {
                                                                                ctx.notifications.show({
                                                                                    content: <ErrorNotification title='Unable to delete project role' e={e} />,
                                                                                    type: NotificationType.Error
                                                                                });
                                                                            }
                                                                        }
                                                                    }}
                                                                    className='argo-button argo-button--base'>
                                                                    Delete
                                                                </button>
                                                            ) : null}
                                                        </div>
                                                    }>
                                                    {(params.get('editRole') !== null || params.get('newRole') === 'true') && (
                                                        <ProjectRoleEditPanel
                                                            nameReadonly={params.get('newRole') === null ? true : false}
                                                            defaultParams={{
                                                                newRole: params.get('newRole') === null ? false : true,
                                                                deleteRole: false,
                                                                projName: proj.metadata.name,
                                                                role:
                                                                    params.get('newRole') === null && proj.spec.roles !== undefined
                                                                        ? proj.spec.roles.find(x => params.get('editRole') === x.name)
                                                                        : undefined,
                                                                jwtTokens:
                                                                    params.get('newRole') === null && proj.spec.roles !== undefined && proj.status.jwtTokensByRole !== undefined
                                                                        ? proj.status.jwtTokensByRole[params.get('editRole')].items
                                                                        : undefined
                                                            }}
                                                            getApi={(api: FormApi) => (this.projectRoleFormApi = api)}
                                                            submit={async (projRoleParams: ProjectRoleParams) => {
                                                                try {
                                                                    await services.projects.updateRole(projRoleParams);
                                                                    ctx.navigation.goto('.', {editRole: null, newRole: null}, {replace: true});
                                                                    this.loader.reload();
                                                                } catch (e) {
                                                                    ctx.notifications.show({
                                                                        content: <ErrorNotification title='Unable to edit project' e={e} />,
                                                                        type: NotificationType.Error
                                                                    });
                                                                }
                                                            }}
                                                            token={this.state.token}
                                                            createJWTToken={async (jwtTokenParams: CreateJWTTokenParams) => this.createJWTToken(jwtTokenParams, ctx.notifications)}
                                                            deleteJWTToken={async (jwtTokenParams: DeleteJWTTokenParams) => this.deleteJWTToken(jwtTokenParams, ctx.notifications)}
                                                            hideJWTToken={() => this.setState({token: ''})}
                                                        />
                                                    )}
                                                </SlidingPanel>
                                                <SlidingPanel
                                                    isNarrow={false}
                                                    isMiddle={false}
                                                    isShown={params.get('editWindow') !== null || params.get('newWindow') !== null}
                                                    onClose={() => {
                                                        this.setState({token: ''});
                                                        ctx.navigation.goto('.', {editWindow: null, newWindow: null}, {replace: true});
                                                    }}
                                                    header={
                                                        <div>
                                                            <button
                                                                onClick={() => {
                                                                    if (params.get('newWindow') === null) {
                                                                        this.projectSyncWindowsFormApi.setValue('id', Number(params.get('editWindow')));
                                                                    }
                                                                    this.projectSyncWindowsFormApi.submitForm(null);
                                                                }}
                                                                className='argo-button argo-button--base'>
                                                                {params.get('newWindow') != null ? 'Create' : 'Update'}
                                                            </button>{' '}
                                                            <button
                                                                onClick={() => {
                                                                    this.setState({token: ''});
                                                                    ctx.navigation.goto('.', {editWindow: null, newWindow: null}, {replace: true});
                                                                }}
                                                                className='argo-button argo-button--base-o'>
                                                                Cancel
                                                            </button>{' '}
                                                            {params.get('newWindow') === null ? (
                                                                <button
                                                                    onClick={async () => {
                                                                        const confirmed = await ctx.popup.confirm(
                                                                            'Delete sync window',
                                                                            'Are you sure you want to delete sync window?'
                                                                        );
                                                                        if (confirmed) {
                                                                            try {
                                                                                this.projectSyncWindowsFormApi.setValue('id', Number(params.get('editWindow')));
                                                                                this.projectSyncWindowsFormApi.setValue('deleteWindow', true);
                                                                                this.projectSyncWindowsFormApi.submitForm(null);
                                                                                ctx.navigation.goto('.', {editWindow: null}, {replace: true});
                                                                            } catch (e) {
                                                                                ctx.notifications.show({
                                                                                    content: <ErrorNotification title='Unable to delete sync window' e={e} />,
                                                                                    type: NotificationType.Error
                                                                                });
                                                                            }
                                                                        }
                                                                    }}
                                                                    className='argo-button argo-button--base'>
                                                                    Delete
                                                                </button>
                                                            ) : null}
                                                        </div>
                                                    }>
                                                    {(params.get('editWindow') !== null || params.get('newWindow') === 'true') && (
                                                        <ProjectSyncWindowsEditPanel
                                                            defaultParams={{
                                                                newWindow: params.get('newWindow') === null ? false : true,
                                                                projName: proj.metadata.name,
                                                                window:
                                                                    params.get('newWindow') === null && proj.spec.syncWindows !== undefined
                                                                        ? proj.spec.syncWindows[Number(params.get('editWindow'))]
                                                                        : undefined,
                                                                id:
                                                                    params.get('newWindow') === null && proj.spec.syncWindows !== undefined
                                                                        ? Number(params.get('editWindow'))
                                                                        : undefined
                                                            }}
                                                            getApi={(api: FormApi) => (this.projectSyncWindowsFormApi = api)}
                                                            submit={async (projectSyncWindowsParams: ProjectSyncWindowsParams) => {
                                                                try {
                                                                    await services.projects.updateWindow(projectSyncWindowsParams);
                                                                    ctx.navigation.goto('.', {editWindow: null, newWindow: null}, {replace: true});
                                                                    this.loader.reload();
                                                                } catch (e) {
                                                                    ctx.notifications.show({
                                                                        content: <ErrorNotification title='Unable to edit project' e={e} />,
                                                                        type: NotificationType.Error
                                                                    });
                                                                }
                                                            }}
                                                        />
                                                    )}
                                                </SlidingPanel>
                                            </div>
                                        );
                                    }}
                                </Query>
                            )}
                        </DataLoader>
                    </Page>
                )}
            </Consumer>
        );
    }

    private async deleteJWTToken(params: DeleteJWTTokenParams, notifications: NotificationsApi) {
        try {
            await services.projects.deleteJWTToken(params);
            const info = await services.projects.getDetailed(this.props.match.params.name);
            this.loader.setData(info);
        } catch (e) {
            notifications.show({
                content: <ErrorNotification title='Unable to delete JWT token' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private async createJWTToken(params: CreateJWTTokenParams, notifications: NotificationsApi) {
        try {
            const jwtToken = await services.projects.createJWTToken(params);
            const info = await services.projects.getDetailed(this.props.match.params.name);
            this.loader.setData(info);
            this.setState({token: jwtToken.token});
        } catch (e) {
            notifications.show({
                content: <ErrorNotification title='Unable to create JWT token' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private eventsTab(proj: Project) {
        return (
            <div className='argo-container'>
                <ProjectEvents projectName={proj.metadata.name} />
            </div>
        );
    }

    private rolesTab(proj: Project, ctx: any) {
        return (
            <div className='argo-container'>
                {((proj.spec.roles || []).length > 0 && (
                    <div className='argo-table-list argo-table-list--clickable'>
                        <div className='argo-table-list__head'>
                            <div className='row'>
                                <div className='columns small-3'>NAME</div>
                                <div className='columns small-6'>DESCRIPTION</div>
                            </div>
                        </div>
                        {(proj.spec.roles || []).map(role => (
                            <div className='argo-table-list__row' key={`${role.name}`} onClick={() => ctx.navigation.goto(`.`, {editRole: role.name})}>
                                <div className='row'>
                                    <div className='columns small-3'>{role.name}</div>
                                    <div className='columns small-6'>{role.description}</div>
                                </div>
                            </div>
                        ))}
                    </div>
                )) || (
                    <div className='white-box'>
                        <p>Project has no roles</p>
                    </div>
                )}
            </div>
        );
    }

    private SyncWindowsTab(proj: Project, ctx: any) {
        return (
            <div className='argo-container'>
                {((proj.spec.syncWindows || []).length > 0 && (
                    <DataLoader
                        noLoaderOnInputChange={true}
                        input={proj.spec.syncWindows}
                        load={async () => {
                            return await services.projects.getSyncWindows(proj.metadata.name);
                        }}>
                        {data => (
                            <div className='argo-table-list argo-table-list--clickable'>
                                <div className='argo-table-list__head'>
                                    <div className='row'>
                                        <div className='columns small-2'>
                                            STATUS
                                            {helpTip(
                                                'If a window is active or inactive and what the current ' +
                                                    'effect would be if it was assigned to an application, namespace or cluster. ' +
                                                    'Red: no syncs allowed. ' +
                                                    'Yellow: manual syncs allowed. ' +
                                                    'Green: all syncs allowed'
                                            )}
                                        </div>
                                        <div className='columns small-2'>
                                            WINDOW
                                            {helpTip('The kind, start time and duration of the window')}
                                        </div>
                                        <div className='columns small-2'>
                                            APPLICATIONS
                                            {helpTip('The applications assigned to the window, wildcards are supported')}
                                        </div>
                                        <div className='columns small-2'>
                                            NAMESPACES
                                            {helpTip('The namespaces assigned to the window, wildcards are supported')}
                                        </div>
                                        <div className='columns small-2'>
                                            CLUSTERS
                                            {helpTip('The clusters assigned to the window, wildcards are supported')}
                                        </div>
                                        <div className='columns small-2'>
                                            MANUALSYNC
                                            {helpTip('If the window allows manual syncs')}
                                        </div>
                                    </div>
                                </div>
                                {(proj.spec.syncWindows || []).map((window, i) => (
                                    <div className='argo-table-list__row' key={`${i}`} onClick={() => ctx.navigation.goto(`.`, {editWindow: `${i}`})}>
                                        <div className='row'>
                                            <div className='columns small-2'>
                                                <span>
                                                    <SyncWindowStatusIcon state={data} window={window} />
                                                </span>
                                            </div>
                                            <div className='columns small-2'>
                                                {window.kind}:{window.schedule}:{window.duration}:{window.timeZone}
                                            </div>
                                            <div className='columns small-2'>{(window.applications || ['-']).join(',')}</div>
                                            <div className='columns small-2'>{(window.namespaces || ['-']).join(',')}</div>
                                            <div className='columns small-2'>{(window.clusters || ['-']).join(',')}</div>
                                            <div className='columns small-2'>{window.manualSync ? 'Enabled' : 'Disabled'}</div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )}
                    </DataLoader>
                )) || (
                    <div className='white-box'>
                        <p>Project has no sync windows</p>
                    </div>
                )}
            </div>
        );
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }

    private async saveProject(updatedProj: Project) {
        try {
            const proj = await services.projects.get(updatedProj.metadata.name);
            proj.metadata.labels = updatedProj.metadata.labels;
            proj.spec = updatedProj.spec;

            await services.projects.update(proj);
            const scopedProj = await services.projects.getDetailed(this.props.match.params.name);
            this.loader.setData(scopedProj);
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to update project' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private summaryTab(proj: Project, globalProj: ProjectSpec & {count: number}, scopedProj: DetailedProjectsResponse) {
        return (
            <div className='argo-container'>
                <EditablePanel
                    save={item => this.saveProject(item)}
                    validate={input => ({
                        'metadata.name': !input.metadata.name && 'Project name is required'
                    })}
                    values={proj}
                    title='GENERAL'
                    items={[
                        {
                            title: 'NAME',
                            view: proj.metadata.name,
                            edit: () => proj.metadata.name
                        },
                        {
                            title: 'DESCRIPTION',
                            view: proj.spec.description,
                            edit: (formApi: FormApi) => <FormField formApi={formApi} field='spec.description' component={Text} />
                        },
                        {
                            title: 'LABELS',
                            view: Object.keys(proj.metadata.labels || {})
                                .map(label => `${label}=${proj.metadata.labels[label]}`)
                                .join(' '),
                            edit: (formApi: FormApi) => <FormField formApi={formApi} field='metadata.labels' component={MapInputField} />
                        },
                        {
                            title: 'LINKS',
                            view: (
                                <div style={{margin: '8px 0'}}>
                                    <DataLoader load={() => services.projects.getLinks(proj.metadata.name)}>{links => <DeepLinks links={links.items} />}</DataLoader>
                                </div>
                            )
                        }
                    ]}
                />

                <EditablePanel
                    save={item => this.saveProject(item)}
                    values={proj}
                    title={<React.Fragment>SOURCE REPOSITORIES {helpTip('Git repositories where application manifests are permitted to be retrieved from')}</React.Fragment>}
                    view={
                        <React.Fragment>
                            {proj.spec.sourceRepos
                                ? proj.spec.sourceRepos.map((repo, i) => (
                                      <div className='row white-box__details-row' key={i}>
                                          <div className='columns small-12'>{repo}</div>
                                      </div>
                                  ))
                                : emptyMessage('source repositories')}
                        </React.Fragment>
                    }
                    edit={formApi => (
                        <DataLoader load={() => services.repos.list()}>
                            {repos => (
                                <React.Fragment>
                                    {(formApi.values.spec.sourceRepos || []).map((_: Project, i: number) => (
                                        <div className='row white-box__details-row' key={i}>
                                            <div className='columns small-12'>
                                                <FormField
                                                    formApi={formApi}
                                                    field={`spec.sourceRepos[${i}]`}
                                                    component={AutocompleteField}
                                                    componentProps={{items: repos.map(repo => repo.repo)}}
                                                />
                                                <i className='fa fa-times' onClick={() => formApi.setValue('spec.sourceRepos', removeEl(formApi.values.spec.sourceRepos, i))} />
                                            </div>
                                        </div>
                                    ))}
                                    <button
                                        className='argo-button argo-button--short'
                                        onClick={() => formApi.setValue('spec.sourceRepos', (formApi.values.spec.sourceRepos || []).concat('*'))}>
                                        ADD SOURCE
                                    </button>
                                </React.Fragment>
                            )}
                        </DataLoader>
                    )}
                    items={[]}
                />

                <EditablePanel
                    values={scopedProj}
                    title={<React.Fragment>SCOPED REPOSITORIES{helpTip('Git repositories where application manifests are permitted to be retrieved from')}</React.Fragment>}
                    view={
                        <React.Fragment>
                            {scopedProj.repositories && scopedProj.repositories.length
                                ? scopedProj.repositories.map((repo, i) => (
                                      <div className='row white-box__details-row' key={i}>
                                          <div className='columns small-12'>{repo.repo}</div>
                                      </div>
                                  ))
                                : emptyMessage('source repositories')}
                        </React.Fragment>
                    }
                    items={[]}
                />
                <AuthSettingsCtx.Consumer>
                    {authCtx =>
                        authCtx.appsInAnyNamespaceEnabled && (
                            <EditablePanel
                                save={item => this.saveProject(item)}
                                values={proj}
                                title={
                                    <React.Fragment>SOURCE NAMESPACES {helpTip('Kubernetes namespaces where application resources are allowed to be created in')}</React.Fragment>
                                }
                                view={
                                    <React.Fragment>
                                        {proj.spec.sourceNamespaces
                                            ? proj.spec.sourceNamespaces.map((namespace, i) => (
                                                  <div className='row white-box__details-row' key={i}>
                                                      <div className='columns small-12'>{namespace}</div>
                                                  </div>
                                              ))
                                            : emptyMessage('source namespaces')}
                                    </React.Fragment>
                                }
                                edit={formApi => (
                                    <React.Fragment>
                                        {(formApi.values.spec.sourceNamespaces || []).map((_: Project, i: number) => (
                                            <div className='row white-box__details-row' key={i}>
                                                <div className='columns small-12'>
                                                    <FormField formApi={formApi} field={`spec.sourceNamespaces[${i}]`} component={AutocompleteField} />
                                                    <i
                                                        className='fa fa-times'
                                                        onClick={() => formApi.setValue('spec.sourceNamespaces', removeEl(formApi.values.spec.sourceNamespaces, i))}
                                                    />
                                                </div>
                                            </div>
                                        ))}
                                        <button
                                            className='argo-button argo-button--short'
                                            onClick={() => formApi.setValue('spec.sourceNamespaces', (formApi.values.spec.sourceNamespaces || []).concat('*'))}>
                                            ADD SOURCE
                                        </button>
                                    </React.Fragment>
                                )}
                                items={[]}
                            />
                        )
                    }
                </AuthSettingsCtx.Consumer>
                <EditablePanel
                    save={item => this.saveProject(item)}
                    values={proj}
                    title={<React.Fragment>DESTINATIONS {helpTip('Cluster and namespaces where applications are permitted to be deployed to')}</React.Fragment>}
                    view={
                        <React.Fragment>
                            {proj.spec.destinations ? (
                                <React.Fragment>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-4'>Server</div>
                                        <div className='columns small-3'>Name</div>
                                        <div className='columns small-5'>Namespace</div>
                                    </div>
                                    {proj.spec.destinations.map((dest, i) => (
                                        <div className='row white-box__details-row' key={i}>
                                            <div className='columns small-4'>{dest.server}</div>
                                            <div className='columns small-3'>{dest.name}</div>
                                            <div className='columns small-5'>{dest.namespace}</div>
                                        </div>
                                    ))}
                                </React.Fragment>
                            ) : (
                                emptyMessage('destinations')
                            )}
                        </React.Fragment>
                    }
                    edit={formApi => (
                        <DataLoader load={() => services.clusters.list()}>
                            {clusters => (
                                <React.Fragment>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-4'>Server</div>
                                        <div className='columns small-3'>Name</div>
                                        <div className='columns small-5'>Namespace</div>
                                    </div>
                                    {(formApi.values.spec.destinations || []).map((_: Project, i: number) => (
                                        <div className='row white-box__details-row' key={i}>
                                            <div className='columns small-4'>
                                                <FormField
                                                    formApi={formApi}
                                                    field={`spec.destinations[${i}].server`}
                                                    component={AutocompleteField}
                                                    componentProps={{items: clusters.map(cluster => cluster.server)}}
                                                />
                                            </div>
                                            <div className='columns small-3'>
                                                <FormField
                                                    formApi={formApi}
                                                    field={`spec.destinations[${i}].name`}
                                                    component={AutocompleteField}
                                                    componentProps={{items: clusters.map(cluster => cluster.name)}}
                                                />
                                            </div>
                                            <div className='columns small-5'>
                                                <FormField formApi={formApi} field={`spec.destinations[${i}].namespace`} component={AutocompleteField} />
                                            </div>
                                            <i className='fa fa-times' onClick={() => formApi.setValue('spec.destinations', removeEl(formApi.values.spec.destinations, i))} />
                                        </div>
                                    ))}
                                    <button
                                        className='argo-button argo-button--short'
                                        onClick={() =>
                                            formApi.setValue(
                                                'spec.destinations',
                                                (formApi.values.spec.destinations || []).concat({
                                                    server: '*',
                                                    namespace: '*',
                                                    name: '*'
                                                })
                                            )
                                        }>
                                        ADD DESTINATION
                                    </button>
                                </React.Fragment>
                            )}
                        </DataLoader>
                    )}
                    items={[]}
                />

                <EditablePanel
                    values={scopedProj}
                    title={<React.Fragment>SCOPED CLUSTERS{helpTip('Cluster and namespaces where applications are permitted to be deployed to')}</React.Fragment>}
                    view={
                        <React.Fragment>
                            {scopedProj.clusters && scopedProj.clusters.length
                                ? scopedProj.clusters.map((cluster, i) => (
                                      <div className='row white-box__details-row' key={i}>
                                          <div className='columns small-12'>{cluster.server}</div>
                                      </div>
                                  ))
                                : emptyMessage('destinations')}
                        </React.Fragment>
                    }
                    items={[]}
                />

                <EditablePanel
                    save={item => this.saveProject(item)}
                    values={proj}
                    title={
                        <React.Fragment>
                            DESTINATION SERVICE ACCOUNTS{' '}
                            {helpTip(
                                'Destination Service Accounts holds information about the service accounts to be impersonated for the application sync operation for each destination.'
                            )}
                        </React.Fragment>
                    }
                    view={
                        <React.Fragment>
                            {proj.spec.destinationServiceAccounts ? (
                                <React.Fragment>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-4'>Server</div>
                                        <div className='columns small-3'>Namespace</div>
                                        <div className='columns small-5'>DefaultServiceAccount</div>
                                    </div>
                                    {proj.spec.destinationServiceAccounts.map((dest, i) => (
                                        <div className='row white-box__details-row' key={i}>
                                            <div className='columns small-4'>{dest.server}</div>
                                            <div className='columns small-3'>{dest.namespace}</div>
                                            <div className='columns small-5'>{dest.defaultServiceAccount}</div>
                                        </div>
                                    ))}
                                </React.Fragment>
                            ) : (
                                emptyMessage('destinationServiceAccount')
                            )}
                        </React.Fragment>
                    }
                    edit={formApi => (
                        <DataLoader load={() => services.clusters.list()}>
                            {clusters => (
                                <React.Fragment>
                                    <div className='row white-box__details-row'>
                                        <div className='columns small-4'>Server</div>
                                        <div className='columns small-3'>Namespace</div>
                                        <div className='columns small-5'>DefaultServiceAccount</div>
                                    </div>
                                    {(formApi.values.spec.destinationServiceAccounts || []).map((_: Project, i: number) => (
                                        <div className='row white-box__details-row' key={i}>
                                            <div className='columns small-4'>
                                                <FormField
                                                    formApi={formApi}
                                                    field={`spec.destinationServiceAccounts[${i}].server`}
                                                    component={AutocompleteField}
                                                    componentProps={{items: clusters.map(cluster => cluster.server)}}
                                                />
                                            </div>
                                            <div className='columns small-3'>
                                                <FormField formApi={formApi} field={`spec.destinationServiceAccounts[${i}].namespace`} component={AutocompleteField} />
                                            </div>
                                            <div className='columns small-5'>
                                                <FormField
                                                    formApi={formApi}
                                                    field={`spec.destinationServiceAccounts[${i}].defaultServiceAccount`}
                                                    component={AutocompleteField}
                                                    componentProps={{items: clusters.map(cluster => cluster.name)}}
                                                />
                                            </div>
                                            <i
                                                className='fa fa-times'
                                                onClick={() => formApi.setValue('spec.destinationServiceAccounts', removeEl(formApi.values.spec.destinationServiceAccounts, i))}
                                            />
                                        </div>
                                    ))}

                                    <button
                                        className='argo-button argo-button--short'
                                        onClick={() =>
                                            formApi.setValue(
                                                'spec.destinationServiceAccounts',
                                                (formApi.values.spec.destinationServiceAccounts || []).concat({
                                                    server: '*',
                                                    namespace: '*',
                                                    defaultServiceAccount: '*'
                                                })
                                            )
                                        }>
                                        ADD DESTINATION SERVICE ACCOUNTS
                                    </button>
                                </React.Fragment>
                            )}
                        </DataLoader>
                    )}
                    items={[]}
                />

                <ResourceListsPanel proj={proj} saveProject={item => this.saveProject(item)} />
                {globalProj.count > 0 && (
                    <ResourceListsPanel
                        title={<p>INHERITED FROM GLOBAL PROJECTS {helpTip('Global projects provide configurations that other projects can inherit from.')}</p>}
                        proj={{metadata: null, spec: globalProj, status: null}}
                    />
                )}

                <EditablePanel
                    save={item => this.saveProject(item)}
                    values={proj}
                    title={<React.Fragment>GPG SIGNATURE KEYS {helpTip('IDs of GnuPG keys that commits must be signed with in order to be allowed to sync to')}</React.Fragment>}
                    view={
                        <React.Fragment>
                            {proj.spec.signatureKeys
                                ? proj.spec.signatureKeys.map((key, i) => (
                                      <div className='row white-box__details-row' key={i}>
                                          <div className='columns small-12'>{key.keyID}</div>
                                      </div>
                                  ))
                                : emptyMessage('signature keys')}
                        </React.Fragment>
                    }
                    edit={formApi => (
                        <DataLoader load={() => services.gpgkeys.list()}>
                            {keys => (
                                <React.Fragment>
                                    {(formApi.values.spec.signatureKeys || []).map((_: Project, i: number) => (
                                        <div className='row white-box__details-row' key={i}>
                                            <div className='columns small-12'>
                                                <FormField
                                                    formApi={formApi}
                                                    field={`spec.signatureKeys[${i}].keyID`}
                                                    component={AutocompleteField}
                                                    componentProps={{items: keys.map(key => key.keyID)}}
                                                />
                                            </div>
                                            <i className='fa fa-times' onClick={() => formApi.setValue('spec.signatureKeys', removeEl(formApi.values.spec.signatureKeys, i))} />
                                        </div>
                                    ))}
                                    <button
                                        className='argo-button argo-button--short'
                                        onClick={() =>
                                            formApi.setValue(
                                                'spec.signatureKeys',
                                                (formApi.values.spec.signatureKeys || []).concat({
                                                    keyID: ''
                                                })
                                            )
                                        }>
                                        ADD KEY
                                    </button>
                                </React.Fragment>
                            )}
                        </DataLoader>
                    )}
                    items={[]}
                />

                <EditablePanel
                    save={item => this.saveProject(item)}
                    values={proj}
                    title={<React.Fragment>RESOURCE MONITORING {helpTip('Enables monitoring of top level resources in the application target namespace')}</React.Fragment>}
                    view={
                        proj.spec.orphanedResources ? (
                            <React.Fragment>
                                <p>
                                    <i className={'fa fa-toggle-on'} /> Enabled
                                </p>
                                <p>
                                    <i
                                        className={classNames('fa', {
                                            'fa-toggle-off': !proj.spec.orphanedResources.warn,
                                            'fa-toggle-on': proj.spec.orphanedResources.warn
                                        })}
                                    />{' '}
                                    Application warning conditions are {proj.spec.orphanedResources.warn ? 'enabled' : 'disabled'}.
                                </p>
                                {(proj.spec.orphanedResources.ignore || []).length > 0 ? (
                                    <React.Fragment>
                                        <p>Resources Ignore List</p>
                                        <div className='row white-box__details-row'>
                                            <div className='columns small-4'>Group</div>
                                            <div className='columns small-4'>Kind</div>
                                            <div className='columns small-4'>Name</div>
                                        </div>
                                        {(proj.spec.orphanedResources.ignore || []).map((resource, i) => (
                                            <div className='row white-box__details-row' key={i}>
                                                <div className='columns small-4'>{resource.group}</div>
                                                <div className='columns small-4'>{resource.kind}</div>
                                                <div className='columns small-4'>{resource.name}</div>
                                            </div>
                                        ))}
                                    </React.Fragment>
                                ) : (
                                    <p>The resource ignore list is empty</p>
                                )}
                            </React.Fragment>
                        ) : (
                            <p>
                                <i className={'fa fa-toggle-off'} /> Disabled
                            </p>
                        )
                    }
                    edit={formApi =>
                        formApi.values.spec.orphanedResources ? (
                            <React.Fragment>
                                <button className='argo-button argo-button--base' onClick={() => formApi.setValue('spec.orphanedResources', null)}>
                                    DISABLE
                                </button>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-4'>
                                        Enable application warning conditions?
                                        <HelpIcon title='If checked, Application will have a warning condition when orphaned resources detected' />
                                    </div>
                                    <div className='columns small-8'>
                                        <FormField formApi={formApi} field='spec.orphanedResources.warn' component={CheckboxField} />
                                    </div>
                                </div>

                                <div>
                                    Resources Ignore List
                                    <HelpIcon title='Define resources that ArgoCD should not report as orphaned' />
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-4'>Group</div>
                                    <div className='columns small-4'>Kind</div>
                                    <div className='columns small-4'>Name</div>
                                </div>
                                {((formApi.values.spec.orphanedResources.ignore || []).length === 0 && <div>Ignore list is empty</div>) ||
                                    formApi.values.spec.orphanedResources.ignore.map((_: Project, i: number) => (
                                        <div className='row white-box__details-row' key={i}>
                                            <div className='columns small-4'>
                                                <FormField
                                                    formApi={formApi}
                                                    field={`spec.orphanedResources.ignore[${i}].group`}
                                                    component={AutocompleteField}
                                                    componentProps={{items: Groups, filterSuggestions: true}}
                                                />
                                            </div>
                                            <div className='columns small-4'>
                                                <FormField
                                                    formApi={formApi}
                                                    field={`spec.orphanedResources.ignore[${i}].kind`}
                                                    component={AutocompleteField}
                                                    componentProps={{items: ResourceKinds, filterSuggestions: true}}
                                                />
                                            </div>
                                            <div className='columns small-4'>
                                                <FormField formApi={formApi} field={`spec.orphanedResources.ignore[${i}].name`} component={AutocompleteField} />
                                            </div>
                                            <i
                                                className='fa fa-times'
                                                onClick={() => formApi.setValue('spec.orphanedResources.ignore', removeEl(formApi.values.spec.orphanedResources.ignore, i))}
                                            />
                                        </div>
                                    ))}
                                <br />
                                <button
                                    className='argo-button argo-button--base'
                                    onClick={() =>
                                        formApi.setValue(
                                            'spec.orphanedResources.ignore',
                                            (formApi.values.spec.orphanedResources ? formApi.values.spec.orphanedResources.ignore || [] : []).concat({
                                                keyID: ''
                                            })
                                        )
                                    }>
                                    ADD RESOURCE
                                </button>
                            </React.Fragment>
                        ) : (
                            <button className='argo-button argo-button--base' onClick={() => formApi.setValue('spec.orphanedResources.ignore', [])}>
                                ENABLE
                            </button>
                        )
                    }
                    items={[]}
                />

                <BadgePanel project={proj.metadata.name} />
            </div>
        );
    }
}
