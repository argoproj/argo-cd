import { NotificationsApi, NotificationType, SlidingPanel, Tabs } from 'argo-ui';
import * as React from 'react';
import { FormApi } from 'react-form';
import { RouteComponentProps } from 'react-router';

import { DataLoader, ErrorNotification, Page, Query } from '../../../shared/components';
import { Consumer } from '../../../shared/context';
import { Project } from '../../../shared/models';
import { CreateJWTTokenParams, DeleteJWTTokenParams, ProjectRoleParams, services } from '../../../shared/services';

import { ProjectEditPanel } from '../project-edit-panel/project-edit-panel';
import { ProjectEvents } from '../project-events/project-events';
import { ProjectRoleEditPanel } from '../project-role-edit-panel/project-role-edit-panel';

interface ProjectDetailsState {
    token: string;
}

export class ProjectDetails extends React.Component<RouteComponentProps<{ name: string; }>, ProjectDetailsState> {
    private projectFormApi: FormApi;
    private projectRoleFormApi: FormApi;
    private loader: DataLoader;

    constructor(props: RouteComponentProps<{ name: string; }>) {
        super(props);
        this.state = {token: ''};
    }

    public render() {
        return (
        <Consumer>
            {(ctx) => (
            <Page title='Projects' toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings' }, {title: 'Projects', path: '/settings/projects'}, {title: this.props.match.params.name}],
                actionMenu: {items: [
                    { title: 'Add Role', iconClassName: 'fa fa-plus', action: () => ctx.navigation.goto('.', {newRole: true})},
                    { title: 'Edit', iconClassName: 'fa fa-pencil-alt', action: () => ctx.navigation.goto('.', {edit: true}) },
                    { title: 'Delete', iconClassName: 'fa fa-times-circle', action: async () => {
                        const confirmed = await ctx.popup.confirm('Delete project', 'Are you sure you want to delete project?');
                        if (confirmed) {
                            try {
                                await services.projects.delete(this.props.match.params.name);
                                ctx.navigation.goto('/settings/projects');
                            } catch (e) {
                                ctx.notifications.show({
                                    content: <ErrorNotification title='Unable to delete project' e={e}/>,
                                    type: NotificationType.Error,
                                });
                            }
                        }
                    },
                }]},
            }}>
                <DataLoader load={() => services.projects.get(this.props.match.params.name)} ref={(loader) => this.loader = loader}>
                    {(proj) => (
                        <Query>
                        {(params) => (
                            <div>
                                <Tabs selectedTabKey={params.get('tab') || 'summary'} onTabSelected={(tab) => ctx.navigation.goto('.', {tab})} navCenter={true} tabs={[{
                                    key: 'summary',
                                    title: 'Summary',
                                    content: this.summaryTab(proj),
                                }, {
                                    key: 'roles',
                                    title: 'Roles',
                                    content: this.rolesTab(proj, ctx),
                                }, {
                                    key: 'events',
                                    title: 'Events',
                                    content: this.eventsTab(proj),
                                }]}/>
                                <SlidingPanel isShown={params.get('edit') === 'true'}
                                    onClose={() => ctx.navigation.goto('.', {edit: null})} header={(
                                    <div>
                                        <button onClick={() => ctx.navigation.goto('.', {edit: null})} className='argo-button argo-button--base-o'>
                                            Cancel
                                        </button> <button onClick={() => this.projectFormApi.submitForm(null)} className='argo-button argo-button--base'>
                                            Update
                                        </button>
                                    </div>
                                )}>
                                    {params.get('edit') === 'true' && <ProjectEditPanel nameReadonly={true} defaultParams={{
                                        name: proj.metadata.name,
                                        description: proj.spec.description,
                                        destinations: proj.spec.destinations || [],
                                        sourceRepos: proj.spec.sourceRepos || [],
                                        clusterResourceWhitelist: proj.spec.clusterResourceWhitelist || [],
                                        namespaceResourceBlacklist: proj.spec.namespaceResourceBlacklist || [],
                                        roles: proj.spec.roles || [],
                                        }} getApi={(api) => this.projectFormApi = api} submit={async (projParams) => {
                                            try {
                                                await services.projects.update(projParams);
                                                ctx.navigation.goto('.', {edit: null});
                                                this.loader.reload();
                                            } catch (e) {
                                                ctx.notifications.show({
                                                    content: <ErrorNotification title='Unable to edit project' e={e}/>,
                                                    type: NotificationType.Error,
                                                });
                                            }
                                        }
                                    }/>}
                                </SlidingPanel>
                                <SlidingPanel isMiddle={true} isShown={params.get('editRole') !== null || params.get('newRole') !== null}
                                    onClose={() => {
                                        this.setState({token: ''});
                                        ctx.navigation.goto('.', {editRole: null, newRole: null});
                                    }} header={(
                                        <div>
                                            <button onClick={() => {
                                                    this.setState({token: ''});
                                                    ctx.navigation.goto('.', {editRole: null, newRole: null});
                                                }} className='argo-button argo-button--base-o'>
                                                Cancel
                                            </button> <button onClick={() => this.projectRoleFormApi.submitForm(null)} className='argo-button argo-button--base'>
                                                {params.get('newRole') != null ? 'Create' : 'Update'}
                                            </button> {params.get('newRole') === null ? (
                                            <button onClick={async () => {
                                                    const confirmed = await ctx.popup.confirm('Delete project role', 'Are you sure you want to delete project role?');
                                                    if (confirmed) {
                                                        try {
                                                            this.projectRoleFormApi.setValue('deleteRole', true);
                                                            this.projectRoleFormApi.submitForm(null);
                                                            ctx.navigation.goto('.', {editRole: null});
                                                        } catch (e) {
                                                            ctx.notifications.show({
                                                                content: <ErrorNotification title='Unable to delete project role' e={e}/>,
                                                                type: NotificationType.Error,
                                                            });
                                                        }
                                                    }
                                                }} className='argo-button argo-button--base'>
                                                    Delete
                                                </button>
                                            ) : null}
                                        </div>
                                    )}>
                                    {(params.get('editRole') !== null || params.get('newRole') === 'true') && <ProjectRoleEditPanel
                                        nameReadonly={params.get('newRole') === null ? true : false}
                                        defaultParams={{
                                            newRole: (params.get('newRole') === null ) ? false : true,
                                            deleteRole: false,
                                            projName: proj.metadata.name,
                                            role: (params.get('newRole') === null && proj.spec.roles !== undefined) ?
                                                proj.spec.roles.find((x) => params.get('editRole') === x.name)
                                                : undefined,
                                        }}
                                        getApi={(api: FormApi) => this.projectRoleFormApi = api} submit={async (projRoleParams: ProjectRoleParams) => {
                                            try {
                                                await services.projects.updateRole(projRoleParams);
                                                ctx.navigation.goto('.', {editRole: null, newRole: null});
                                                this.loader.reload();
                                            } catch (e) {
                                                ctx.notifications.show({
                                                    content: <ErrorNotification title='Unable to edit project' e={e}/>,
                                                    type: NotificationType.Error,
                                                });
                                            }
                                        }}
                                        token={this.state.token}
                                        createJWTToken={async (jwtTokenParams: CreateJWTTokenParams) => this.createJWTToken(jwtTokenParams, ctx.notifications)}
                                        deleteJWTToken={async (jwtTokenParams: DeleteJWTTokenParams) => this.deleteJWTToken(jwtTokenParams, ctx.notifications)}
                                        hideJWTToken={() => this.setState({token: ''})}
                                    />}
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
                content: <ErrorNotification title='Unable to delete JWT token' e={e}/>,
                type: NotificationType.Error,
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
                content: <ErrorNotification title='Unable to create JWT token' e={e}/>,
                type: NotificationType.Error,
            });
        }
    }

    private eventsTab(proj: Project) {
        return (
            <div className='argo-container'>
                <ProjectEvents projectName={proj.metadata.name}/>
            </div>
        );
    }

    private rolesTab(proj: Project, ctx: any) {
        return (
            <div className='argo-container'>
                {(proj.spec.roles || []).length > 0 && (
                    <div className='argo-table-list argo-table-list--clickable'>
                        <div className='argo-table-list__head'>
                            <div className='row'>
                                <div className='columns small-3'>NAME</div>
                                <div className='columns small-6'>DESCRIPTION</div>
                            </div>
                        </div>
                        {(proj.spec.roles || []).map((role) => (
                            <div className='argo-table-list__row' key={`${role.name}`} onClick={() => ctx.navigation.goto(`.`, {editRole: role.name})}>
                                <div className='row'>
                                    <div className='columns small-3'>
                                        {role.name}
                                    </div>
                                    <div className='columns small-6'>
                                        {role.description}
                                    </div>
                                </div>
                            </div>
                        ))}
                </div>) || <div className='white-box'><p>Project has no roles</p></div>}
            </div>
        );
    }

    private summaryTab(proj: Project) {
        const attributes = [
            {title: 'NAME', value: proj.metadata.name},
            {title: 'DESCRIPTION', value: proj.spec.description},
        ];
        return (
            <div className='argo-container'>
                <div className='white-box'>
                <div className='white-box__details'>
                    {attributes.map((attr) => (
                        <div className='row white-box__details-row' key={attr.title}>
                            <div className='columns small-3'>
                                {attr.title}
                            </div>
                            <div className='columns small-9'>{attr.value}</div>
                        </div>
                    ))}
                </div>
            </div>

            <h4>Source repositories</h4>
            {(proj.spec.sourceRepos || []).length > 0 && (
            <div className='argo-table-list'>
                <div className='argo-table-list__head'>
                    <div className='row'>
                        <div className='columns small-12'>URL</div>
                    </div>
                </div>
                {(proj.spec.sourceRepos || []).map((src) => (
                    <div className='argo-table-list__row' key={src}>
                        <div className='row'>
                            <div className='columns small-12'>
                                {src}
                            </div>
                        </div>
                    </div>
                ))}
            </div>) || <div className='white-box'><p>Project has no source repositories</p></div>}

            <h4>Destinations</h4>
            {(proj.spec.destinations || []).length > 0 && (
            <div className='argo-table-list'>
                <div className='argo-table-list__head'>
                    <div className='row'>
                        <div className='columns small-3'>SERVER</div>
                        <div className='columns small-6'>NAMESPACE</div>
                    </div>
                </div>
                {(proj.spec.destinations || []).map((dst) => (
                    <div className='argo-table-list__row' key={`${dst.server}/${dst.namespace}`}>
                        <div className='row'>
                            <div className='columns small-3'>
                                {dst.server}
                            </div>
                            <div className='columns small-6'>
                                {dst.namespace}
                            </div>
                        </div>
                    </div>
                ))}
            </div>) || <div className='white-box'><p>Project has no destinations</p></div>}

            <h4>Whitelisted cluster resources</h4>
            {(proj.spec.clusterResourceWhitelist || []).length > 0 && (
            <div className='argo-table-list'>
                <div className='argo-table-list__head'>
                    <div className='row'>
                        <div className='columns small-3'>GROUP</div>
                        <div className='columns small-6'>KIND</div>
                    </div>
                </div>
                {(proj.spec.clusterResourceWhitelist || []).map((res) => (
                    <div className='argo-table-list__row' key={`${res.group}/${res.kind}`}>
                        <div className='row'>
                            <div className='columns small-3'>
                                {res.group}
                            </div>
                            <div className='columns small-6'>
                                {res.kind}
                            </div>
                        </div>
                    </div>
                ))}
            </div>) || <div className='white-box'><p>No cluster-scoped resources are permitted to deploy</p></div>}

            <h4>Blacklisted namespaced resources</h4>
            {(proj.spec.namespaceResourceBlacklist || []).length > 0 && (
            <div className='argo-table-list'>
                <div className='argo-table-list__head'>
                    <div className='row'>
                        <div className='columns small-3'>GROUP</div>
                        <div className='columns small-6'>KIND</div>
                    </div>
                </div>
                {(proj.spec.namespaceResourceBlacklist || []).map((res) => (
                    <div className='argo-table-list__row' key={`${res.group}/${res.kind}`}>
                        <div className='row'>
                            <div className='columns small-3'>
                                {res.group}
                            </div>
                            <div className='columns small-6'>
                                {res.kind}
                            </div>
                        </div>
                    </div>
                ))}
            </div>) || <div className='white-box'><p>All namespaced-scoped resources are permitted to deploy</p></div>}
        </div>
        );
    }
}
