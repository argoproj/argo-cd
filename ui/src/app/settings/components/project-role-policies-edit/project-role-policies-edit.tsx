import * as ReactForm from 'react-form';
import React from 'react';

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
}

function generatePolicy(project: string, role: string, resource?: string, action?: string, object?: string, permission?: string): string {
    return `p, proj:${project}:${role}, ${resource || ''}, ${action || ''}, ${object ? project + '/' + object : ''}, ${permission || ''}`;
}

const actions = ['get', 'create', 'update', 'delete', 'sync', 'override'];

export const ProjectRolePoliciesEdit = (props: ProjectRolePoliciesProps) => (
    <DataLoader load={() => services.applications.list([props.projName], {fields: ['items.metadata.name']}).then(list => list.items)}>
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

const PolicyWrapper = ({projName, roleName, fieldApi, actions, availableApps, deletePolicy}: PolicyProps) => {
    const getResource = (): string => {
        const fields = (fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            return '';
        }
        return fields[2].trim();
    };

    const setResource = (resource: string) => {
        const fields = (fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            fieldApi.setValue(generatePolicy(projName, roleName, resource, '', '', ''));
            return;
        }
        fields[2] = ` ${resource}`;
        fieldApi.setValue(fields.join());
    };

    const getAction = (): string => {
        const fields = (fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            return '';
        }
        return fields[3].trim();
    };

    const setAction = (action: string) => {
        const fields = (fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            fieldApi.setValue(generatePolicy(projName, roleName, '', action, '', ''));
            return;
        }
        fields[3] = ` ${action}`;
        fieldApi.setValue(fields.join());
    };

    const getObject = (): string => {
        const fields = (fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            return '';
        }
        return fields[4].trim();
    };

    const setObject = (object: string) => {
        const fields = (fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            fieldApi.setValue(generatePolicy(projName, roleName, '', '', object, ''));
            return;
        }
        fields[4] = ` ${object}`;
        fieldApi.setValue(fields.join());
    };

    const getPermission = (): string => {
        const fields = (fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            return '';
        }
        return fields[5].trim();
    };

    const setPermission = (permission: string) => {
        const fields = (fieldApi.getValue() as string).split(',');
        if (fields.length !== 6) {
            fieldApi.setValue(generatePolicy(projName, roleName, '', '', '', permission));
            return;
        }
        fields[5] = ` ${permission}`;
        fieldApi.setValue(fields.join());
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
                    {actions !== undefined && actions.length > 0 && actions.map(action => <option key={action}>{action}</option>)}
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
                    {availableApps !== undefined &&
                        availableApps.length > 0 &&
                        availableApps.map(app => (
                            <option key={app.metadata.name}>
                                {projName}/{app.metadata.name}
                            </option>
                        ))}
                    <option key='wildcard'>{`${projName}/*`}</option>
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
                <i className='fa fa-times' onClick={deletePolicy} style={{cursor: 'pointer'}} />
            </div>
        </div>
    );
};

const Policy = ReactForm.FormField(PolicyWrapper);
