import * as React from 'react';
import { DataLoader, Page } from '../../../shared/components';
import { services } from '../../../shared/services';

export const ProjectsList = () => (
    <Page title='Projects' toolbar={{ breadcrumbs: [{title: 'Settings', path: '/settings' }, {title: 'Projects'}] }}>
        <div className='projects argo-container'>
            <DataLoader load={() => services.projects.list()}>
                {(projects) => (
                    projects.length > 0 && (
                        <div className='argo-table-list'>
                            <div className='argo-table-list__head'>
                                <div className='row'>
                                    <div className='columns small-3'>NAME</div>
                                    <div className='columns small-6'>DESCRIPTION</div>
                                </div>
                            </div>
                            {projects.map((proj) => (
                                <div className='argo-table-list__row' key={proj.metadata.name}>
                                    <div className='row'>
                                        <div className='columns small-3'>
                                            <i className='fa fa-object-group'/> {proj.metadata.name}
                                        </div>
                                        <div className='columns small-6'>
                                            {proj.spec.description}
                                        </div>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )
                )}
            </DataLoader>
        </div>
    </Page>
);
