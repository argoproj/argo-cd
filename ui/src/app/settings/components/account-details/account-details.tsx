import {ErrorNotification, FormField, NotificationType} from 'argo-ui';
import {useQuery, useQueryClient} from '@tanstack/react-query';
import * as React from 'react';
import {Form, Text} from 'argo-ui';
import {RouteComponentProps} from 'react-router';

import {Page, Timestamp} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {Token} from '../../../shared/models';
import {services} from '../../../shared/services';
import {accountQueries} from '../../../shared/queries';
import {loadFailed, loading} from '../../../shared/query-client';

import {convertExpiresInToSeconds, validExpiresIn} from '../utils';

require('./account-details.scss');

export const AccountDetails = (props: RouteComponentProps<{name: string}>) => {
    const ctx = React.useContext(Context);
    const [newToken, setNewToken] = React.useState(null);
    const queryClient = useQueryClient();
    const {data: account, isError, refetch} = useQuery(accountQueries.detail(props.match.params.name));
    const tokens: Token[] = account?.tokens || [];
    return (
        <Page
            title={props.match.params.name}
            toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Accounts', path: '/settings/accounts'}, {title: props.match.params.name}]
            }}>
            <p />
            <div className='argo-container account-details'>
                {isError ? (
                    loadFailed(refetch)
                ) : !account ? (
                    loading
                ) : (
                    <React.Fragment>
                        <div className='white-box'>
                            <div className='white-box__details'>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>NAME</div>
                                    <div className='columns small-9'>{account.name}</div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>ENABLED</div>
                                    <div className='columns small-9'>{(account.enabled && 'true') || 'false'}</div>
                                </div>
                                <div className='row white-box__details-row'>
                                    <div className='columns small-3'>CAPABILITIES</div>
                                    <div className='columns small-9'>{(account.capabilities || []).join(', ')}</div>
                                </div>
                            </div>
                        </div>

                        <h4>Tokens</h4>
                        <Form
                            onSubmit={async (params, event, api) => {
                                const expiresIn = convertExpiresInToSeconds(params.expiresIn);
                                const confirmed = await ctx.popup.confirm('Generate new token', 'Are you sure you want to generate a new token?');
                                if (!confirmed) {
                                    return;
                                }
                                try {
                                    setNewToken(await services.accounts.createToken(props.match.params.name, params.id, expiresIn));
                                    api.resetAll();
                                    await queryClient.invalidateQueries({queryKey: accountQueries.detail(props.match.params.name).queryKey});
                                } catch (e) {
                                    ctx.notifications.show({
                                        content: <ErrorNotification title='Unable to generate new token' e={e} />,
                                        type: NotificationType.Error
                                    });
                                }
                            }}
                            validateError={params => ({
                                expiresIn: !validExpiresIn(params.expiresIn) && 'Must be in the "[0-9]+[smhd]" format'
                            })}>
                            {api => (
                                <form onSubmit={api.submitForm}>
                                    <div className='row argo-table-list__row'>
                                        <div className='columns small-10'>
                                            <div className='argo-form-row'>
                                                <FormField formApi={api} label='Token ID' field='id' component={Text} />
                                            </div>
                                            <div className='argo-form-row'>
                                                <FormField formApi={api} label='Expires In' field='expiresIn' component={Text} />
                                            </div>
                                        </div>
                                        <div className='columns small-2'>
                                            <div className='argo-form-row'>
                                                <a onClick={() => api.submitForm(null)}>Generate New</a>
                                            </div>
                                        </div>
                                    </div>
                                </form>
                            )}
                        </Form>
                        {newToken && (
                            <div className='white-box account-details__new-token'>
                                <h5>New Token:</h5>
                                <p>{newToken}</p>
                                <i className='fa fa-times account-details__remove-token' title='Remove' onClick={() => setNewToken(null)} />
                            </div>
                        )}
                        {(tokens.length > 0 && (
                            <div className='argo-table-list'>
                                <div className='argo-table-list__head'>
                                    <div className='row'>
                                        <div className='columns small-4'>ID</div>
                                        <div className='columns small-4'>ISSUED AT</div>
                                        <div className='columns small-4'>EXPIRES AT</div>
                                    </div>
                                </div>
                                {tokens.map(token => (
                                    <div className='argo-table-list__row' key={token.id}>
                                        <div className='row'>
                                            <div className='columns small-4'>{token.id}</div>
                                            <div className='columns small-4'>
                                                <Timestamp date={token.issuedAt * 1000} />
                                            </div>
                                            <div className='columns small-4'>
                                                {(token.expiresAt && <Timestamp date={token.expiresAt * 1000} />) || <span>Never</span>}
                                                <i
                                                    className='fa fa-times account-details__remove-token'
                                                    title='Delete'
                                                    onClick={async () => {
                                                        const confirmed = await ctx.popup.confirm('Delete Token?', `Are you sure you want to delete token '${token.id}?'`);
                                                        if (!confirmed) {
                                                            return;
                                                        }

                                                        try {
                                                            await services.accounts.deleteToken(props.match.params.name, token.id);
                                                            await queryClient.invalidateQueries({queryKey: accountQueries.detail(props.match.params.name).queryKey});
                                                        } catch (e) {
                                                            ctx.notifications.show({
                                                                content: <ErrorNotification title='Unable to delete token' e={e} />,
                                                                type: NotificationType.Error
                                                            });
                                                        }
                                                    }}
                                                />
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )) || (
                            <div className='white-box'>
                                <p>Account has no tokens. Click 'Generate New' to create one.</p>
                            </div>
                        )}
                    </React.Fragment>
                )}
            </div>
        </Page>
    );
};
