import * as React from 'react';
import * as ReactForm from 'react-form';

import {DataLoader} from '../../../shared/components';
import {Application} from '../../../shared/models';
import {services} from '../../../shared/services';

require('./project-role-policies-edit.scss');

interface ProjectRolePoliciesProps {
    projName: string;
    roleName: string;
    policies: string[];
    formApi: ReactForm.FormApi;
    newRole: boolean;
    objectListKind?: string;
}

function generatePolicy(project: string, role: string, resource?: string, action?: string, object?: string, permission?: string): string {
    return `p, proj:${project}:${role}, ${resource || ''}, ${action || ''}, ${object ? project + '/' + object : ''}, ${permission || ''}`;
}

const actions = ['get', 'create', 'update', 'delete', 'sync', 'override'];

export const ProjectRolePoliciesEdit = (props: ProjectRolePoliciesProps) => {
    const objectListKind = props.objectListKind || 'application';
    return (
        <DataLoader load={() => services.applications.list([props.projName], objectListKind, {fields: ['items.metadata.name']}).then(list => list.items)}>
            {applications => (
                <React.Fragment>
                    <p>POLICY RULES</p>
                    <div>Manage this role's permissions to applications, appsets, repositories, clusters, exec and logs</div>
                    <div className='argo-table-list'>
                        <div className='argo-table-list__head'>
                            <div className='row'>
                                <div className='columns small-3'>RESOURCE</div>
                                <div className='columns small-3'>ACTION</div>
                                <div className='columns small-3'>OBJECT</div>
                                <div className='columns small-3'>PERMISSION</div>
                            </div>
                        </div>
                        <div className='argo-table-list__row'>
                            {props.policies.map((policy, i) => (
                                <Policy
                                    key={i}
                                    field={['policies', i]}
                                    formApi={props.formApi}
                                    policy={policy}
                                    projName={props.projName}
                                    roleName={props.roleName}
                                    deletePolicy={() => props.formApi.setValue('policies', removeEl(props.policies, i))}
                                    availableApps={applications}
                                    actions={actions}
                                />
                            ))}
                            <div className='row'>
                                <div className='columns small-4'>
                                    <a
                                        className='argo-button argo-button--base'
                                        onClick={() => {
                                            const newPolicy = generatePolicy(props.projName, props.roleName);
                                            props.formApi.setValue('policies', (props.formApi.values.policies || []).concat(newPolicy));
                                        }}>
                                        Add policy
                                    </a>
                                </div>
                            </div>
                        </div>
                    </div>
                </React.Fragment>
            )}
        </DataLoader>
    );
};

interface PolicyProps {
    projName: string;
    roleName: string;
    policy: string;
    fieldApi: ReactForm.FieldApi;
    actions: string[];
    availableApps: Application[];
    deletePolicy: () => void;
}

function removeEl(items: any[], index: number) {
    items.splice(index, 1);
    return items;
}

function PolicyWrapper(props: PolicyProps) {
    const getResource = (): string => {
        const fields = (props.fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            return '';
        }
        return fields[2].trim();
    };

    const setResource = (resource: string) => {
        const fields = (props.fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            props.fieldApi.setValue(generatePolicy(props.projName, props.roleName, resource, '', '', ''));
            return;
        }
        fields[2] = ` ${resource}`;
        props.fieldApi.setValue(fields.join());
    };

    const getAction = (): string => {
        const fields = (props.fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            return '';
        }
        return fields[3].trim();
    };

    const setAction = (action: string) => {
        const fields = (props.fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            props.fieldApi.setValue(generatePolicy(props.projName, props.roleName, '', action, '', ''));
            return;
        }
        fields[3] = ` ${action}`;
        props.fieldApi.setValue(fields.join());
    };

    const getObject = (): string => {
        const fields = (props.fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            return '';
        }
        return fields[4].trim();
    };

    const setObject = (object: string) => {
        const fields = (props.fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            props.fieldApi.setValue(generatePolicy(props.projName, props.roleName, '', '', object, ''));
            return;
        }
        fields[4] = ` ${object}`;
        props.fieldApi.setValue(fields.join());
    };

    const getPermission = (): string => {
        const fields = (props.fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            return '';
        }
        return fields[5].trim();
    };
    const setPermission = (permission: string) => {
        const fields = (props.fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            props.fieldApi.setValue(generatePolicy(props.projName, props.roleName, '', '', '', permission));
            return;
        }
        fields[5] = ` ${permission}`;
        props.fieldApi.setValue(fields.join());
    };

    return (
        <div className='row project-role-policies-edit__wrapper-row'>
            <div className='columns small-3'>
                <datalist id='resource'>
                    <option>applications</option>
                    <option>applicationsets</option>
                    <option>clusters</option>
                    <option>repositories</option>
                    <option>logs</option>
                    <option>exec</option>
                </datalist>
                <input
                    className='argo-field'
                    list='resource'
                    value={getResource()}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                        setResource(e.target.value);
                    }}
                />
            </div>
            <div className='columns small-3'>
                <datalist id='action'>
                    {props.actions !== undefined && props.actions.length > 0 && props.actions.map(action => <option key={action}>{action}</option>)}
                    <option key='wildcard'>*</option>
                </datalist>
                <input
                    className='argo-field'
                    list='action'
                    value={getAction()}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                        setAction(e.target.value);
                    }}
                />
            </div>
            <div className='columns small-3'>
                <datalist id='object'>
                    {props.availableApps !== undefined &&
                        props.availableApps.length > 0 &&
                        props.availableApps.map(app => (
                            <option key={app.metadata.name}>
                                {props.projName}/{app.metadata.name}
                            </option>
                        ))}
                    <option key='wildcard'>{`${props.projName}/*`}</option>
                </datalist>
                <input
                    className='argo-field'
                    list='object'
                    value={getObject()}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                        setObject(e.target.value);
                    }}
                />
            </div>
            <div className='columns small-3'>
                <datalist id='permission'>
                    <option>allow</option>
                    <option>deny</option>
                </datalist>
                <input
                    className='argo-field'
                    list='permission'
                    value={getPermission()}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                        setPermission(e.target.value);
                    }}
                />
            </div>
            <div style={{position: 'absolute', right: '0.5em'}}>
                <i className='fa fa-times' onClick={() => props.deletePolicy()} style={{cursor: 'pointer'}} />
            </div>
        </div>
    );
}

const Policy = ReactForm.FormField(PolicyWrapper);
