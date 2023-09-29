import {FormField, Tooltip} from 'argo-ui';
import * as React from 'react';
import {Form, FormApi, Text} from 'react-form';

import {Consumer, ContextApis} from '../../../shared/context';
import {JwtToken} from '../../../shared/models';
import {CreateJWTTokenParams, DeleteJWTTokenParams} from '../../../shared/services';
import {convertExpiresInToSeconds, validExpiresIn} from '../utils';

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
            {ctx => (
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
                            {props.tokens.map(jwToken => renderJWTRow(props, ctx, jwToken))}
                        </div>
                    )}
                    <Form
                        getApi={props.getApi}
                        defaultValues={{expiresIn: ''}}
                        validateError={(params: any) => ({
                            expiresIn: !validExpiresIn(params.expiresIn) && 'Must be in the "[0-9]+[smhd]" format. For example, "12h", "7d".'
                        })}>
                        {api => (
                            <form onSubmit={api.submitForm} role='form' className='width-control'>
                                <div className='white-box'>
                                    <div className='argo-table-list'>
                                        <div className='argo-form-row'>
                                            <FormField formApi={api} label='Token ID' field='id' component={Text} />
                                        </div>
                                        <div className='argo-form-row'>
                                            <FormField formApi={api} label='Expires In' field='expiresIn' component={Text} />
                                        </div>

                                        <div>
                                            <button className='argo-button argo-button--base' onClick={() => createJWTToken(props, api, ctx)}>
                                                Create
                                            </button>
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
                                                    <i className='fa fa-times project-role-jwt-tokens__hide-token' onClick={() => props.hideJWTToken()} />
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
    const id = api.values.id;
    const nameContext = id === undefined || id === '' ? ` has no token ID and ` : ` has token ID '${id}' and `;
    const confirmed = await ctx.popup.confirm(
        'Create JWT Token',
        `Are you sure you want to create a JWT token that ${nameContext}${expiresInPrompt} for role '${role}' in project '${project}'?`
    );
    if (confirmed) {
        props.createJWTToken({project, role, expiresIn, id} as CreateJWTTokenParams);
        api.values.expiresIn = '';
        api.values.id = '';
    }
}

async function deleteJWTToken(props: ProjectRoleJWTTokensProps, iat: number, ctx: any, id: string) {
    const confirmed = await ctx.popup.confirm('Delete JWT Token', `Are you sure you want to delete token ID '${id}' for role '${props.roleName}' in project '${props.projName}'?`);
    if (confirmed) {
        props.deleteJWTToken({project: props.projName, role: props.roleName, iat} as DeleteJWTTokenParams);
    }
}

function renderJWTRow(props: ProjectRoleJWTTokensProps, ctx: ContextApis, jwToken: JwtToken): React.ReactFragment {
    const issuedAt = new Date(jwToken.iat * 1000).toISOString();
    const expiresAt = jwToken.exp == null ? 'Never' : new Date(jwToken.exp * 1000).toISOString();

    return (
        <React.Fragment>
            <div className='argo-table-list__row' key={`${jwToken.iat}`}>
                <div className='row'>
                    <Tooltip content={jwToken.id}>
                        <div className='columns small-3'>{jwToken.id}</div>
                    </Tooltip>
                    <Tooltip content={issuedAt}>
                        <div className='columns small-4'>{issuedAt}</div>
                    </Tooltip>
                    <Tooltip content={expiresAt}>
                        <div className='columns small-4'>{expiresAt}</div>
                    </Tooltip>
                    <Tooltip content='Delete Token'>
                        <div className='columns small-1'>
                            <i className='fa fa-times' onClick={() => deleteJWTToken(props, jwToken.iat, ctx, jwToken.id)} style={{cursor: 'pointer'}} />
                        </div>
                    </Tooltip>
                </div>
            </div>
        </React.Fragment>
    );
}
