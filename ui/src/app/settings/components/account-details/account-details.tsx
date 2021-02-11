import {ErrorNotification, FormField, NotificationType} from 'argo-ui';
import * as React from 'react';
import {Form, Text} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {DataLoader, Page, Timestamp} from '../../../shared/components';
import {Context} from '../../../shared/context';
import {Account, Token} from '../../../shared/models';
import {services} from '../../../shared/services';

import {convertExpiresInToSeconds, validExpiresIn} from '../utils';

require('./account-details.scss');

export const AccountDetails = (props: RouteComponentProps<{name: string}>) => {
    const ctx = React.useContext(Context);
    const [newToken, setNewToken] = React.useState(null);
    const tokensLoaderRef = React.useRef<DataLoader>();
    return (
        <Page
            title={props.match.params.name}
            toolbar={{
                breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'Accounts', path: '/settings/accounts'}, {title: props.match.params.name}]
            }}>
            <p />
            <div className='argo-container account-details'>
                <DataLoader input={props.match.params.name} load={(name: string) => services.accounts.get(name)}>
                    {(account: Account) => (
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
                                        <div className='columns small-9'>{account.capabilities.join(', ')}</div>
                                    </div>
                                </div>
                            </div>

                            <h4>Tokens</h4>
                            <Form
                                onSubmit={async (params, event, api) => {
                                    const expiresIn = convertExpiresInToSeconds(params.expiresIn);
                                    const confirmed = await ctx.popup.confirm('Generate new token?', 'Are you sure you want to generate new token?');
                                    if (!confirmed) {
                                        return;
                                    }
                                    try {
                                        setNewToken(await services.accounts.createToken(props.match.params.name, params.id, expiresIn));
                                        api.resetAll();
                                        if (tokensLoaderRef.current) {
                                            tokensLoaderRef.current.reload();
                                        }
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
                            <DataLoader ref={tokensLoaderRef} input={props.match.params.name} load={(name: string) => services.accounts.get(name).then(acc => acc.tokens || [])}>
                                {(tokens: Token[]) =>
                                    (tokens.length > 0 && (
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
                                                                    const confirmed = await ctx.popup.confirm(
                                                                        'Delete Token?',
                                                                        `Are you sure you want to delete token '${token.id}?'`
                                                                    );
                                                                    if (!confirmed) {
                                                                        return;
                                                                    }

                                                                    try {
                                                                        await services.accounts.deleteToken(props.match.params.name, token.id);
                                                                        if (tokensLoaderRef.current) {
                                                                            tokensLoaderRef.current.reload();
                                                                        }
                                                                    } catch (e) {
                                                                        ctx.notifications.show({
                                                                            content: <ErrorNotification title='Unable to delete token token' e={e} />,
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
                                    )
                                }
                            </DataLoader>
                        </React.Fragment>
                    )}
                </DataLoader>
            </div>
        </Page>
    );
};
