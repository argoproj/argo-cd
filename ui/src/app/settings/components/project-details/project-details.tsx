import {NotificationsApi, NotificationType, SlidingPanel, Tabs} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {DataLoader, ErrorNotification, Page, Query} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import {Project} from '../../../shared/models';
import {CreateJWTTokenParams, DeleteJWTTokenParams, ProjectRoleParams, services} from '../../../shared/services';

import {ProjectEvents} from '../project-events/project-events';

import {ProjectRoleEditPanel} from '../project-role-edit-panel/project-role-edit-panel';
import {ProjectSyncWindowsEditPanel} from '../project-sync-windows-edit-panel/project-sync-windows-edit-panel';

import {ProjectSummary} from '../project/summary/summary';

import {ProjectSyncWindowsParams} from '../../../shared/services/projects-service';

import {SyncWindowStatusIcon} from '../../../applications/components/utils';
import {HelpTip} from '../project/card/card';

interface ProjectDetailsState {
    token: string;
}

export class ProjectDetails extends React.Component<RouteComponentProps<{name: string}>, ProjectDetailsState> {
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
                                    {title: 'Add Role', iconClassName: 'fa fa-plus', action: () => ctx.navigation.goto('.', {newRole: true})},
                                    {title: 'Add Sync Window', iconClassName: 'fa fa-plus', action: () => ctx.navigation.goto('.', {newWindow: true})},
                                    {
                                        title: 'Delete',
                                        iconClassName: 'fa fa-times-circle',
                                        action: async () => {
                                            const confirmed = await ctx.popup.confirm('Delete project', 'Are you sure you want to delete project?');
                                            if (confirmed) {
                                                try {
                                                    await services.projects.delete(this.props.match.params.name);
                                                    ctx.navigation.goto('/settings/projects');
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
                        <DataLoader load={() => services.projects.get(this.props.match.params.name)} ref={loader => (this.loader = loader)}>
                            {proj => (
                                <Query>
                                    {params => (
                                        <div>
                                            <Tabs
                                                selectedTabKey={params.get('tab') || 'summary'}
                                                onTabSelected={tab => ctx.navigation.goto('.', {tab})}
                                                navCenter={true}
                                                tabs={[
                                                    {
                                                        key: 'summary',
                                                        title: 'Summary',
                                                        content: this.summaryTab(proj)
                                                    },
                                                    {
                                                        key: 'roles',
                                                        title: 'Roles',
                                                        content: this.rolesTab(proj, ctx)
                                                    },
                                                    {
                                                        key: 'windows',
                                                        title: 'Windows',
                                                        content: this.SyncWindowsTab(proj, ctx)
                                                    },
                                                    {
                                                        key: 'events',
                                                        title: 'Events',
                                                        content: this.eventsTab(proj)
                                                    }
                                                ]}
                                            />
                                            <SlidingPanel
                                                isMiddle={true}
                                                isShown={params.get('editRole') !== null || params.get('newRole') !== null}
                                                onClose={() => {
                                                    this.setState({token: ''});
                                                    ctx.navigation.goto('.', {editRole: null, newRole: null});
                                                }}
                                                header={
                                                    <div>
                                                        <button
                                                            onClick={() => {
                                                                this.setState({token: ''});
                                                                ctx.navigation.goto('.', {editRole: null, newRole: null});
                                                            }}
                                                            className='argo-button argo-button--base-o'>
                                                            Cancel
                                                        </button>{' '}
                                                        <button onClick={() => this.projectRoleFormApi.submitForm(null)} className='argo-button argo-button--base'>
                                                            {params.get('newRole') != null ? 'Create' : 'Update'}
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
                                                                            ctx.navigation.goto('.', {editRole: null});
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
                                                                ctx.navigation.goto('.', {editRole: null, newRole: null});
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
                                                    ctx.navigation.goto('.', {editWindow: null, newWindow: null});
                                                }}
                                                header={
                                                    <div>
                                                        <button
                                                            onClick={() => {
                                                                this.setState({token: ''});
                                                                ctx.navigation.goto('.', {editWindow: null, newWindow: null});
                                                            }}
                                                            className='argo-button argo-button--base-o'>
                                                            Cancel
                                                        </button>{' '}
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
                                                        {params.get('newWindow') === null ? (
                                                            <button
                                                                onClick={async () => {
                                                                    const confirmed = await ctx.popup.confirm('Delete sync window', 'Are you sure you want to delete sync window?');
                                                                    if (confirmed) {
                                                                        try {
                                                                            this.projectSyncWindowsFormApi.setValue('id', Number(params.get('editWindow')));
                                                                            this.projectSyncWindowsFormApi.setValue('deleteWindow', true);
                                                                            this.projectSyncWindowsFormApi.submitForm(null);
                                                                            ctx.navigation.goto('.', {editWindow: null});
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
                                                                ctx.navigation.goto('.', {editWindow: null, newWindow: null});
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
                                    )}
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
            this.loader.setData(await services.projects.get(this.props.match.params.name));
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
            this.loader.setData(await services.projects.get(this.props.match.params.name));
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
                                            {HelpTip(
                                                'If a window is active or inactive and what the current ' +
                                                    'effect would be if it was assigned to an application, namespace or cluster. ' +
                                                    'Red: no syncs allowed. ' +
                                                    'Yellow: manual syncs allowed. ' +
                                                    'Green: all syncs allowed'
                                            )}
                                        </div>
                                        <div className='columns small-2'>
                                            WINDOW
                                            {HelpTip('The kind, start time and duration of the window')}
                                        </div>
                                        <div className='columns small-2'>
                                            APPLICATIONS
                                            {HelpTip('The applications assigned to the window, wildcards are supported')}
                                        </div>
                                        <div className='columns small-2'>
                                            NAMESPACES
                                            {HelpTip('The namespaces assigned to the window, wildcards are supported')}
                                        </div>
                                        <div className='columns small-2'>
                                            CLUSTERS
                                            {HelpTip('The clusters assigned to the window, wildcards are supported')}
                                        </div>
                                        <div className='columns small-2'>
                                            MANUALSYNC
                                            {HelpTip('If the window allows manual syncs')}
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
                                                {window.kind}:{window.schedule}:{window.duration}
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

    private summaryTab(proj: Project) {
        return <ProjectSummary proj={proj} />;
    }
}
