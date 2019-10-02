import { FormField } from 'argo-ui';
import * as React from 'react';
import {Form, FormApi, Text} from 'react-form';

import * as models from '../../../shared/models';

import { ProjectMaintenanceParams } from '../../../shared/services/projects-service';
import {
    ProjectMaintenanceWindowsApplicationsEdit,
    ProjectMaintenanceWindowsClusterEdit,
    ProjectMaintenanceWindowsNamespaceEdit,
} from '../project-maintenance-windows-edit/project-maintenance-windows-edit';

interface ProjectMaintenanceDefaultParams {
    projName: string;
    window: models.ProjectMaintenanceWindow;
    newWindow: boolean;
}

interface ProjectMaintenanceEditPanelProps {
    nameReadonly?: boolean;
    submit: (params: ProjectMaintenanceParams) => any;
    getApi?: (formApi: FormApi) => void;
    defaultParams: ProjectMaintenanceDefaultParams;
}

export const ProjectMaintenanceEditPanel = (props: ProjectMaintenanceEditPanelProps) => {
    if (props.defaultParams.window === undefined) {
        const w = {} as models.ProjectMaintenanceWindow;
        props.defaultParams.window = w;

    }
    return (
        <div className='project-maintenance-edit-panel'>
        <Form
            onSubmit={props.submit}
            getApi={props.getApi}
            defaultValues={{
                projName: props.defaultParams.projName,
                window: props.defaultParams.window,
            }}
            validateError={(params: ProjectMaintenanceParams) => ({
                projName: !params.projName && 'Project name is required',
                window: !params.window && 'Window is required',
            })
            }>
            {(api) => (
                <form onSubmit={api.submitForm} role='form' className='width-control'>
                    <div className='argo-form-row'>
                        <FormField formApi={api} label='Schedule (in crontab format, e.g. "0 8 * * *" would be 8am daily)'
                               componentProps={{ readOnly: props.nameReadonly}} field='window.schedule' component={Text}/>
                    </div>
                    <div className='argo-form-row'>
                         <FormField formApi={api} label='Duration (e.g. "30m" or "1h")'
                               componentProps={{ readOnly: props.nameReadonly}} field='window.duration' component={Text}/>
                    </div>
                    <ProjectMaintenanceWindowsApplicationsEdit
                        projName={api.values.projName}
                        window={api.values.window}
                        formApi={api}/>
                    <ProjectMaintenanceWindowsNamespaceEdit
                        projName={api.values.projName}
                        window={api.values.window}
                        formApi={api}/>
                    <ProjectMaintenanceWindowsClusterEdit
                        projName={api.values.projName}
                        window={api.values.window}
                        formApi={api}/>
                </form>
            )}
        </Form>
        </div>
    );
};
