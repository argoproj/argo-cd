import { FormField } from 'argo-ui';
import * as React from 'react';
import { Form, FormApi, Text } from 'react-form';

import { Consumer } from '../../../shared/context';
import { JwtToken } from '../../../shared/models';
import { CreateJWTTokenParams, DeleteJWTTokenParams } from '../../../shared/services';

interface ProjectRoleJWTTokensProps {
    projName: string;
    roleName: string;
    tokens: JwtToken[];
    token: string;
    createJWTToken: (params: CreateJWTTokenParams) => void;
    deleteJWTToken: (params: DeleteJWTTokenParams) => void;
    hideJWTToken: () => void;
    getApi?: (formApi: FormApi) => void;
}

require('./project-role-jwt-tokens.scss');

export const ProjectRoleJWTTokens = (props: ProjectRoleJWTTokensProps) => {
        return (
            <Consumer>
            {(ctx) => (
                <React.Fragment>
                <h4>JWT Tokens</h4>
                <div>Generate JWT tokens to bind to this role</div>
                {props.tokens && props.tokens.length > 0 && (
                    <div className='argo-table-list'>
                    <div className='argo-table-list__head'>
                        <div className='row'>
                            <div className='columns small-3'>ID</div>
                            <div className='columns small-4'>ISSUED AT</div>
                            <div className='columns small-4'>EXPIRES AT</div>
                        </div>
                    </div>
                    {props.tokens.map((jwtToken: JwtToken) => (
                        <div className='argo-table-list__row' key={`${jwtToken.iat}`}>
                            <div className='row'>
                                <div className='columns small-3'>
                                    {jwtToken.iat}
                                </div>
                                <div className='columns small-4'>
                                    {new Date(jwtToken.iat * 1000).toISOString()}
                                </div>
                                <div className='columns small-4'>
                                    {jwtToken.exp == null ? 'None' : new Date(jwtToken.exp * 1000).toISOString()}
                                </div>
                                <div className='columns small-1'>
                                    <i className='fa fa-times' onClick={() => deleteJWTToken(props, jwtToken.iat, ctx)} style={{cursor: 'pointer'}}/>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
                )}
                <Form
                    getApi={props.getApi}
                    defaultValues={{ expiresIn: ''}}
                    validateError={(params: any) => ({
                        expiresIn: !validExpiresIn(params.expiresIn) && 'Must be in the "[0-9]+[smhd]" format',
                    })}>
                        {(api) => (
                            <form onSubmit={api.submitForm} role='form' className='width-control'>
                                <div className='argo-table-list'>
                                    <div className='argo-table-list__head'>
                                        <div className='row'>
                                            <div className='columns small-3'>EXPIRES IN</div>
                                        </div>
                                    </div>
                                    <div className='argo-table-list__row'>
                                        <div className='row'>
                                            <div className='columns small-9'>
                                            <FormField formApi={api} label='' field='expiresIn' component={Text}/>
                                            </div>
                                            <div className='columns small-3'>
                                            <a onClick={() => createJWTToken(props, api, ctx)}>Create token</a>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                                {props.token && (
                                <div className='argo-table-list'>
                                    <div className='argo-table-list__head'>
                                        <div className='row'>
                                            <div className='columns small-3'>NEW TOKEN</div>
                                        </div>
                                    </div>
                                    <div className='argo-table-list__row'>
                                        <div className='white-box'>
                                        <p style={{overflowWrap: 'break-word'}}>
                                            {props.token}
                                            <i className='fa fa-times project-role-jwt-tokens__hide-token' onClick={() => props.hideJWTToken()}/>
                                        </p>
                                        </div>
                                    </div>
                                </div>
                                )}
                            </form>
                        )}
                </Form>
                </React.Fragment>
            )}
            </Consumer>
        );
};

function convertExpiresInToSeconds(expiresIn: string): number {
    if (!expiresIn) {
        return 0;
    }
    const time = expiresIn.match('^([0-9]+)([smhd])$');
    const duration = parseInt(time[1], 10);
    let interval = 1;
    if (time[2] === 'm') {
        interval = 60;
    } else if (time[2] === 'h') {
        interval = 60 * 60;
    } else if  (time[2] === 'd') {
        interval = 60 * 60 * 24;
    }
    return duration * interval;
}

async function createJWTToken(props: ProjectRoleJWTTokensProps, api: FormApi, ctx: any) {
    if (api.errors.expiresIn) {
        return;
    }
    const project = props.projName;
    const role = props.roleName;
    const expiresIn = convertExpiresInToSeconds(api.values.expiresIn);
    let expiresInPrompt = 'has no expiration';
    if (expiresIn !== 0) {
        expiresInPrompt = 'expires in ' + api.values.expiresIn;
    }
    const confirmed = await ctx.popup.confirm(
        'Create JWT Token', `Are you sure you want to create a JWT token that ${expiresInPrompt} for role '${role}' in project '${project}'?`);
    if (confirmed) {
        props.createJWTToken({project, role, expiresIn} as CreateJWTTokenParams);
        api.values.expiresIn = '';
    }
}

async function deleteJWTToken(props: ProjectRoleJWTTokensProps, iat: number, ctx: any) {
    const confirmed = await ctx.popup.confirm(
        'Delete JWT Token', `Are you sure you want to delete ID '${iat}' for role '${props.roleName}' in project '${props.projName}'?`);
    if (confirmed) {
        props.deleteJWTToken({project: props.projName, role: props.roleName, iat} as DeleteJWTTokenParams);
    }
}

function validExpiresIn(expiresIn: string): boolean {
    if (!expiresIn) {
        return true;
    }
    return expiresIn.match('^([0-9]+)([smhd])$') !== null;
}
