import * as React from 'react';
import * as ReactForm from 'react-form';
import {ProjectMaintenanceWindow} from '../../../shared/models';

interface ProjectMaintenanceWindowsProps {
    projName: string;
    window: ProjectMaintenanceWindow;
    formApi: ReactForm.FormApi;
}

export const ProjectMaintenanceWindowsApplicationsEdit = (props: ProjectMaintenanceWindowsProps) => (
    <React.Fragment>
        <h4>Applications</h4>
        <div>Manage applications assigned to this window</div>
        <div className='argo-table-list__row'>
            {(props.window.applications || []).map((a, i) => (
                <Attribute key={i} field={['window.applications', i]}
                        formApi={props.formApi}
                        projName={props.projName}
                        deleteApp={() => props.formApi.setValue('window.applications', removeEl(props.window.applications, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a onClick={() => {
                        const newA = '';
                        props.formApi.setValue('window.applications', (props.formApi.values.window.applications || []).concat(newA));
                    }}>Add Application</a>
                </div>
            </div>
        </div>
    </React.Fragment>
);

export const ProjectMaintenanceWindowsNamespaceEdit = (props: ProjectMaintenanceWindowsProps) => (
    <React.Fragment>
        <h4>Namespaces</h4>
        <div>Manage namespaces assigned to this window</div>
        <div className='argo-table-list__row'>
            {(props.window.namespaces || []).map((n, i) => (
                <Attribute key={i} field={['window.namespaces', i]}
                           formApi={props.formApi}
                           projName={props.projName}
                           deleteApp={() => props.formApi.setValue('window.namespaces', removeEl(props.window.namespaces, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a onClick={() => {
                        const newN = '';
                        props.formApi.setValue('window.namespaces', (props.formApi.values.window.namespaces || []).concat(newN));
                    }}>Add Namespace</a>
                </div>
            </div>
        </div>
    </React.Fragment>
);

export const ProjectMaintenanceWindowsClusterEdit = (props: ProjectMaintenanceWindowsProps) => (
    <React.Fragment>
        <h4>Namespaces</h4>
        <div>Manage namespaces assigned to this window</div>
        <div className='argo-table-list__row'>
            {(props.window.clusters || []).map((c, i) => (
                <Attribute key={i} field={['window.clusters', i]}
                           formApi={props.formApi}
                           projName={props.projName}
                           deleteApp={() => props.formApi.setValue('window.clusters', removeEl(props.window.clusters, i))}
                />
            ))}
            <div className='row'>
                <div className='columns small-6'>
                    <a onClick={() => {
                        const newC = '';
                        props.formApi.setValue('window.clusters', (props.formApi.values.window.clusters || []).concat(newC));
                    }}>Add Cluster</a>
                </div>
            </div>
        </div>
    </React.Fragment>
);

interface AttributeProps {
    projName: string;
    roleName: string;
    fieldApi: ReactForm.FieldApi;
    deleteApp: () => void;
}

function removeEl(items: any[], index: number) {
    items.splice(index, 1);
    return items;
}

class AttributeWrapper extends React.Component<AttributeProps, any> {

    public render() {
        return (
                <div className='row'>
                    <div className='columns small-6'>
                        <input className='argo-field' value={this.getApplication()} onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                            this.setApplication(e.target.value);
                        }}/>
                    </div>
                    <div className='columns small-1'>
                        <i className='fa fa-times' onClick={() => this.props.deleteApp()} style={{cursor: 'pointer'}}/>
                    </div>
                </div>
        );
    }

    private getApplication(): string {
        return this.props.fieldApi.getValue();
    }

    private setApplication(application: string) {
        this.props.fieldApi.setValue(application);
    }
}

const Attribute = ReactForm.FormField(AttributeWrapper);
