import * as React from 'react';
import { Form, FormApi, Text, TextArea } from 'react-form';

import { FormField } from '../../../shared/components';
import * as models from '../../../shared/models';
import { ProjectRoleParams } from '../../../shared/services';

interface ProjectRoleDefaultParams {
    projName: string;
    role?: models.ProjectRole;
    deleteRole: boolean;
}

interface ProjectRoleEditPanelProps {
    nameReadonly?: boolean;
    submit: (params: ProjectRoleParams) => any;
    getApi?: (formApi: FormApi) => void;
    defaultParams: ProjectRoleDefaultParams;
}

export class ProjectRoleEditPanel extends React.Component<ProjectRoleEditPanelProps, any> {

    public render() {
        return (
            <div className='project-role-edit-panel'>
            <Form
                onSubmit={this.props.submit}
                getApi={this.props.getApi}
                defaultValues={{
                    projName: this.props.defaultParams.projName,
                    roleName: (this.props.defaultParams.role !== undefined ? this.props.defaultParams.role.name : ''),
                    description: (this.props.defaultParams.role !== undefined ? this.props.defaultParams.role.description : ''),
                    policies: (this.props.defaultParams.role !== undefined && this.props.defaultParams.role.policies !== null
                        ? this.props.defaultParams.role.policies.join('\n') : ''),
                    jwtTokens: (this.props.defaultParams.role !== undefined ? this.props.defaultParams.role.jwtTokens : []),
                }}
                validateError={(params: ProjectRoleParams) => ({
                    projName: !params.projName && 'Project name is required',
                    roleName: !params.roleName && 'Role name is required',
                })
                }>
                {(api) => (
                    <form onSubmit={api.submitForm} role='form' className='width-control'>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Role Name'
                            componentProps={{ readOnly: this.props.nameReadonly }} field='roleName' component={Text}/>
                        </div>
                        <div className='argo-form-row'>
                            <FormField formApi={api} label='Role Description' field='description' component={Text}/>
                        </div>
                        <h4>Policies:</h4>
                        <FormField formApi={api} label='' field='policies' component={TextArea}/>
                        <h4>JWT Tokens:</h4>
                        { api.values.jwtTokens !== null && api.values.jwtTokens.length > 0 ? (
                            <div className='argo-table-list'>
                                <div className='argo-table-list__head'>
                                    <div className='row'>
                                        <div className='columns small-3'>ID</div>
                                        <div className='columns small-3'>ISSUED AT</div>
                                        <div className='columns small-3'>EXPIRES AT</div>
                                    </div>
                                </div>
                                {api.values.jwtTokens.map((jwtToken: models.JwtToken) => (
                                    <div className='argo-table-list__row' key={`${jwtToken.iat}`}>
                                        <div className='row'>
                                            <div className='columns small-3'>
                                                {jwtToken.iat}
                                            </div>
                                            <div className='columns small-3'>
                                                {new Date(jwtToken.iat * 1000).toDateString()}
                                            </div>
                                            <div className='columns small-3'>
                                                {jwtToken.exp == null ? 'None' : new Date(jwtToken.exp * 1000).toDateString()}
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        ) : <div className='white-box'><p>Role has no JWT tokens</p></div> }
                    </form>
                )}
            </Form>
            </div>
        );
    }
}
