import {FormField} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi, Text} from 'react-form';

import * as models from '../../../shared/models';
import {CreateJWTTokenParams, DeleteJWTTokenParams, ProjectRoleParams} from '../../../shared/services';
import {ProjectRoleGroupsEdit} from '../project-role-groups-edit/project-role-groups-edit';
import {ProjectRoleJWTTokens} from '../project-role-jwt-tokens/project-role-jwt-tokens';
import {ProjectRolePoliciesEdit} from '../project-role-policies-edit/project-role-policies-edit';

interface ProjectRoleDefaultParams {
    projName: string;
    role?: models.ProjectRole;
    newRole: boolean;
    deleteRole: boolean;
    jwtTokens: models.JwtToken[];
}

interface ProjectRoleEditPanelProps {
    nameReadonly?: boolean;
    submit: (params: ProjectRoleParams) => any;
    createJWTToken: (params: CreateJWTTokenParams) => void;
    deleteJWTToken: (params: DeleteJWTTokenParams) => void;
    hideJWTToken: () => void;
    token: string;
    getApi?: (formApi: FormApi) => void;
    defaultParams: ProjectRoleDefaultParams;
}

export const ProjectRoleEditPanel = (props: ProjectRoleEditPanelProps) => {
    return (
        <div className='project-role-edit-panel'>
            <Form
                onSubmit={props.submit}
                getApi={props.getApi}
                defaultValues={{
                    projName: props.defaultParams.projName,
                    roleName: (props.defaultParams.role && props.defaultParams.role.name) || '',
                    description: (props.defaultParams.role && props.defaultParams.role.description) || '',
                    policies: (props.defaultParams.role && props.defaultParams.role.policies) || [],
                    jwtTokens: (props.defaultParams.role && props.defaultParams.jwtTokens) || [],
                    groups: (props.defaultParams.role && props.defaultParams.role.groups) || []
                }}
                validateError={(params: ProjectRoleParams) => ({
                    projName: !params.projName && 'Project name is required',
                    roleName: !params.roleName && 'Role name is required'
                })}>
                {api => (
                    <form onSubmit={api.submitForm} role='form' className='width-control'>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Role Name' componentProps={{readOnly: props.nameReadonly}} field='roleName' component={Text} />
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Role Description' field='description' component={Text} />
                        </div>
                        <ProjectRolePoliciesEdit
                            projName={api.values.projName}
                            roleName={api.values.roleName}
                            formApi={api}
                            policies={api.values.policies}
                            newRole={props.defaultParams.newRole}
                        />
                        <ProjectRoleGroupsEdit
                            projName={api.values.projName}
                            roleName={api.values.roleName}
                            formApi={api}
                            groups={api.values.groups}
                            newRole={props.defaultParams.newRole}
                        />
                    </form>
                )}
            </Form>
            {!props.defaultParams.newRole && props.defaultParams.role !== undefined ? (
                <ProjectRoleJWTTokens
                    projName={props.defaultParams.projName}
                    roleName={props.defaultParams.role.name}
                    tokens={props.defaultParams.jwtTokens as models.JwtToken[]}
                    token={props.token}
                    createJWTToken={props.createJWTToken}
                    deleteJWTToken={props.deleteJWTToken}
                    hideJWTToken={props.hideJWTToken}
                />
            ) : null}
        </div>
    );
};
