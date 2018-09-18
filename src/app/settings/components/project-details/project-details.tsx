import { NotificationType, SlidingPanel, Tabs } from 'argo-ui';
import * as React from 'react';
import { FormApi } from 'react-form';
import { RouteComponentProps } from 'react-router';

import { DataLoader, ErrorNotification, Page, Query } from '../../../shared/components';
import { Consumer } from '../../../shared/context';
import { Project } from '../../../shared/models';
import { ProjectRoleParams, services } from '../../../shared/services';

import { ProjectEditPanel } from '../project-edit-panel/project-edit-panel';
import { ProjectEvents } from '../project-events/project-events';
import { ProjectRoleEditPanel } from '../project-role-edit-panel/project-role-edit-panel';

export class ProjectDetails extends React.Component<RouteComponentProps<{ name: string; }>> {
    private projectFormApi: FormApi;
    private projectRoleFormApi: FormApi;
    private loader: DataLoader;

    public render() {
        return (
        <Consumer>
            {(ctx) => (
            <Page title='Projects' toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings' }, {title: 'Projects', path: '/settings/projects'}, {title: this.props.match.params.name}],
                actionMenu: {items: [
                    { title: 'Add Role', iconClassName: 'icon fa fa-plus', action: () => {
                        this.projectRoleFormApi.setValue('roleName', '');
                        this.projectRoleFormApi.setValue('description', '');
                        this.projectRoleFormApi.setValue('policies', '');
                        this.projectRoleFormApi.setValue('jwtTokens', []);
                        ctx.navigation.goto('.', {newRole: true});
                    }},
                    { title: 'Edit', iconClassName: 'icon fa fa-pencil', action: () => ctx.navigation.goto('.', {edit: true}) },
                    { title: 'Delete', iconClassName: 'icon fa fa-times-circle', action: async () => {
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
                                    key: 'events',
                                    title: 'Events',
                                    content: this.eventsTab(proj),
                                }, {
                                    key: 'roles',
                                    title: 'Roles',
                                    content: this.rolesTab(proj, ctx),
                                }]}/>
                                <SlidingPanel isMiddle={true} isShown={params.get('edit') === 'true'}
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
                                    onClose={() => ctx.navigation.goto('.', {editRole: null, newRole: null})} header={(
                                        <div>
                                            <button onClick={() => ctx.navigation.goto('.', {editRole: null, newRole: null})} className='argo-button argo-button--base-o'>
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
                                    <ProjectRoleEditPanel nameReadonly={params.get('newRole') === null ? true : false}
                                        defaultParams={{
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
                                    />
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
                            <div className='argo-table-list__row' key={`${role.name}`}
                            onClick={() => {
                                this.projectRoleFormApi.setValue('roleName', role.name);
                                this.projectRoleFormApi.setValue('description', role.description);
                                this.projectRoleFormApi.setValue('policies', role.policies !== null ? role.policies.join('\n') : '');
                                this.projectRoleFormApi.setValue('jwtTokens', role.jwtTokens);
                                ctx.navigation.goto(`.`, {editRole: role.name});
                            }}>
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
        </div>
        );
    }
}
