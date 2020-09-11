import * as React from 'react';

import {DataLoader, EmptyState, Page, Query} from '../../../shared/components';
import {Consumer} from '../../../shared/context';
import {services} from '../../../shared/services';
import {ProjectCreate} from './create/create';

export class ProjectsList extends React.Component {
    public render() {
        return (
            <Consumer>
                {ctx => (
                    <Page
                        title='Projects'
                        toolbar={{
                            breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Projects'}],
                            actionMenu: {className: 'fa fa-plus', items: [{title: 'New Project', action: () => ctx.navigation.goto('.', {add: true})}]}
                        }}>
                        <div className='projects argo-container'>
                            <DataLoader load={() => services.projects.list()}>
                                {projects =>
                                    (projects.length > 0 && (
                                        <div className='argo-table-list argo-table-list--clickable'>
                                            <div className='argo-table-list__head'>
                                                <div className='row'>
                                                    <div className='columns small-3'>NAME</div>
                                                    <div className='columns small-6'>DESCRIPTION</div>
                                                </div>
                                            </div>
                                            {projects.map(proj => (
                                                <div className='argo-table-list__row' key={proj.metadata.name} onClick={() => ctx.navigation.goto(`./${proj.metadata.name}`)}>
                                                    <div className='row'>
                                                        <div className='columns small-3'>
                                                            <i className='fa fa-object-group' /> {proj.metadata.name}
                                                        </div>
                                                        <div className='columns small-6'>{proj.spec.description}</div>
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    )) || (
                                        <EmptyState icon='fa fa-object-group'>
                                            <h4>No projects yet</h4>
                                            <h5>Create new projects to group your applications</h5>
                                            <button className='argo-button argo-button--base' onClick={() => ctx.navigation.goto('.', {add: true})}>
                                                New project
                                            </button>
                                        </EmptyState>
                                    )
                                }
                            </DataLoader>
                        </div>
                        <Query>
                            {params => (
                                <div>
                                    {params.get('add') === 'true' ? (
                                        <div>
                                            <ProjectCreate />
                                            <div onClick={() => ctx.navigation.goto('.', {add: null})}>Close</div>
                                        </div>
                                    ) : null}
                                </div>
                            )}
                        </Query>
                    </Page>
                )}
            </Consumer>
        );
    }
}
