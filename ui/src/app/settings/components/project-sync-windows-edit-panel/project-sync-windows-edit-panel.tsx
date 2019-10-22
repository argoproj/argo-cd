import {FormField, FormSelect} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi, Text} from 'react-form';

import {CheckboxField} from '../../../shared/components';

import * as models from '../../../shared/models';

import {ProjectSyncWindowsParams} from '../../../shared/services/projects-service';
import {
    ProjectSyncWindowApplicationsEdit,
    ProjectSyncWindowClusterEdit,
    ProjectSyncWindowNamespaceEdit,
    ProjectSyncWindowScheduleEdit
} from '../project-sync-windows-edit/project-sync-windows-edit';

interface ProjectSyncWindowsDefaultParams {
    projName: string;
    window: models.SyncWindow;
    newWindow: boolean;
    id: number;
}

interface ProjectSyncWindowsEditPanelProps {
    submit: (params: ProjectSyncWindowsParams) => any;
    getApi?: (formApi: FormApi) => void;
    defaultParams: ProjectSyncWindowsDefaultParams;
}

export const ProjectSyncWindowsEditPanel = (props: ProjectSyncWindowsEditPanelProps) => {
    if (props.defaultParams.window === undefined) {
        const w = {
            schedule: '* * * * *'
        } as models.SyncWindow;
        props.defaultParams.window = w;
    }
    return (
        <div className='project-sync-windows-edit-panel'>
            <Form
                onSubmit={props.submit}
                getApi={props.getApi}
                defaultValues={{
                    projName: props.defaultParams.projName,
                    window: props.defaultParams.window
                }}
                validateError={(params: ProjectSyncWindowsParams) => ({
                    projName: !params.projName && 'Project name is required',
                    window: !params.window && 'Window is required'
                })}>
                {api => (
                    <form onSubmit={api.submitForm} role='form' className='width-control'>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Kind' componentProps={{options: ['allow', 'deny']}} field='window.kind' component={FormSelect} />
                        </div>
                        <ProjectSyncWindowScheduleEdit projName={api.values.projName} window={api.values.window} formApi={api} />
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Duration (e.g. "30m" or "1h")' field='window.duration' component={Text} />
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Enable manual sync' field='window.manualSync' component={CheckboxField} />
                        </div>
                        <ProjectSyncWindowApplicationsEdit projName={api.values.projName} window={api.values.window} formApi={api} />
                        <ProjectSyncWindowNamespaceEdit projName={api.values.projName} window={api.values.window} formApi={api} />
                        <ProjectSyncWindowClusterEdit projName={api.values.projName} window={api.values.window} formApi={api} />
                    </form>
                )}
            </Form>
        </div>
    );
};
