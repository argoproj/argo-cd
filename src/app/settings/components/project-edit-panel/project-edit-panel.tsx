import * as React from 'react';
import { Form, FormApi, Text } from 'react-form';

import { DataLoader, FormField, Select } from '../../../shared/components';
import * as models from '../../../shared/models';
import { ProjectParams, services } from '../../../shared/services';

require('./project-edit-panel.scss');

function removeEl(items: any[], index: number) {
    items.splice(index, 1);
    return items;
}

export const ProjectEditPanel = (props: {
    nameReadonly?: boolean,
    defaultParams?: ProjectParams,
    submit: (params: ProjectParams) => any,
    getApi?: (formApi: FormApi) => void,
}) => (
    <div className='project-edit-panel'>
    <Form
        onSubmit={props.submit}
        getApi={props.getApi}
        defaultValues={{sourceRepos: [], destinations: [], ...props.defaultParams}}
        validateError={(params: ProjectParams) => ({
            name: !params.name && 'Project name is required',
        })}>

        {(api) => (
            <form onSubmit={api.submitForm} role='form' className='width-control'>
                <h4>Summary:</h4>
                <div className='argo-form-row'>
                    <FormField formApi={api} label='Project Name' componentProps={{ readOnly: props.nameReadonly }} field='name' component={Text}/>
                </div>
                <div className='argo-form-row'>
                    <FormField formApi={api} label='Project Description' field='description' component={Text}/>
                </div>
                <DataLoader load={() => services.reposService.list().then((repos) => repos.concat({repo: '*'} as models.Repository).map((repo) => repo.repo))}>
                    {(repos) => (
                        <React.Fragment>
                        <h4>Sources:</h4>
                        {(api.values.sourceRepos as Array<string>).map((_, i) => (
                            <div key={i} className='row project-edit-panel__form-row'>
                                <div className='columns small-12'>
                                    <Select field={['sourceRepos', i]} options={repos}/>
                                    <i className='fa fa-times' onClick={() => api.setValue('sourceRepos', removeEl(api.values.sourceRepos, i))}/>
                                </div>
                            </div>
                        ))}
                        <a onClick={() => api.setValue('sourceRepos', api.values.sourceRepos.concat(repos[0]))}>add source</a>
                        </React.Fragment>
                    )}
                </DataLoader>

                <DataLoader load={() => services.clustersService.list().then((clusters) => clusters.concat({server: '*'} as models.Cluster).map((cluster) => cluster.server))}>
                    {(clusters) => (
                        <React.Fragment>
                            <h4>Destinations:</h4>
                            {(api.values.destinations as Array<models.ApplicationDestination>).map((_, i) => (
                                <div key={i} className='row project-edit-panel__form-row'>
                                    <div className='columns small-5'>
                                        <Select field={['destinations', i, 'server']} options={clusters}/>
                                    </div>
                                    <div className='columns small-5'>
                                        <Text className='argo-field' field={['destinations', i, 'namespace']}/>
                                    </div>
                                    <div className='columns small-2'>
                                        <i className='fa fa-times' onClick={() => api.setValue('destinations', removeEl(api.values.destinations, i))}/>
                                    </div>
                                </div>
                            ))}
                            <a onClick={() => api.setValue('destinations', api.values.destinations.concat({ server: clusters[0], namespace: 'default' }))}>add destination</a>
                        </React.Fragment>
                    )}
                </DataLoader>
            </form>
        )}
    </Form>
    </div>
);
