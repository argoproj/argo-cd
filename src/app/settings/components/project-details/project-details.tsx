import { NotificationType, SlidingPanel, Tabs } from 'argo-ui';
import * as React from 'react';
import { FormApi } from 'react-form';
import { RouteComponentProps } from 'react-router';

import { DataLoader, ErrorNotification, Page, Query } from '../../../shared/components';
import { Consumer } from '../../../shared/context';
import { Project } from '../../../shared/models';
import { services } from '../../../shared/services';

import { ProjectEditPanel } from '../project-edit-panel/project-edit-panel';
import { ProjectEvents } from '../project-events/project-events';

export class ProjectDetails extends React.Component<RouteComponentProps<{ name: string; }>> {
    private formApi: FormApi;
    private loader: DataLoader;

    public render() {
        return (
        <Consumer>
            {(ctx) => (
            <Page title='Projects' toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings' }, {title: 'Projects', path: '/settings/projects'}, {title: this.props.match.params.name}],
                actionMenu: {items: [
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
                                }]}/>
                                <SlidingPanel isMiddle={true} isShown={params.get('edit') === 'true'} onClose={() => ctx.navigation.goto('.', {edit: null})} header={(
                                    <div>
                                        <button onClick={() => ctx.navigation.goto('.', {edit: null})} className='argo-button argo-button--base-o'>
                                            Cancel
                                        </button> <button onClick={() => this.formApi.submitForm(null)} className='argo-button argo-button--base'>
                                            Update
                                        </button>
                                    </div>
                                )}>
                                <ProjectEditPanel nameReadonly={true} defaultParams={{
                                    name: proj.metadata.name,
                                    description: proj.spec.description,
                                    destinations: proj.spec.destinations || [],
                                    sourceRepos: proj.spec.sourceRepos || [],
                                }} getApi={(api) => this.formApi = api} submit={async (projParams) => {
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
                                }}/>
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
