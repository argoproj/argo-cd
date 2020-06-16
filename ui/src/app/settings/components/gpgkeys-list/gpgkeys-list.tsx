import {DropDownMenu, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import * as PropTypes from 'prop-types';
import * as React from 'react';
import {Form, FormApi, TextArea} from 'react-form';
import {RouteComponentProps} from 'react-router';

import {DataLoader, EmptyState, ErrorNotification, Page} from '../../../shared/components';
import {AppContext} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

require('./gpgkeys-list.scss');

interface NewGnuPGPublicKeyParams {
    keyData: string;
}

export class GpgKeysList extends React.Component<RouteComponentProps<any>> {
    public static contextTypes = {
        router: PropTypes.object,
        apis: PropTypes.object,
        history: PropTypes.object
    };

    private formApi: FormApi;
    private loader: DataLoader;

    public render() {
        return (
            <Page
                title='GnuPG public keys'
                toolbar={{
                    breadcrumbs: [{title: 'Settings', path: '/settings'}, {title: 'GnuPG public keys'}],
                    actionMenu: {
                        className: 'fa fa-plus',
                        items: [
                            {
                                title: 'Add GnuPG key',
                                action: () => (this.showAddGnuPGKey = true)
                            }
                        ]
                    }
                }}>
                <div className='gpgkeys-list'>
                    <div className='argo-container'>
                        <DataLoader load={() => services.gpgkeys.list()} ref={loader => (this.loader = loader)}>
                            {(gpgkeys: models.GnuPGPublicKey[]) =>
                                (gpgkeys.length > 0 && (
                                    <div className='argo-table-list'>
                                        <div className='argo-table-list__head'>
                                            <div className='row'>
                                                <div className='columns small-3'>KEY ID</div>
                                                <div className='columns small-3'>KEY TYPE</div>
                                                <div className='columns small-6'>IDENTITY</div>
                                            </div>
                                        </div>
                                        {gpgkeys.map(gpgkey => (
                                            <div className='argo-table-list__row' key={gpgkey.keyID}>
                                                <div className='row'>
                                                    <div className='columns small-3'>
                                                        <i className='fa fa-key' /> {gpgkey.keyID}
                                                    </div>
                                                    <div className='columns small-3'>{gpgkey.subType.toUpperCase()}</div>
                                                    <div className='columns small-6'>
                                                        {gpgkey.owner}
                                                        <DropDownMenu
                                                            anchor={() => (
                                                                <button className='argo-button argo-button--light argo-button--lg argo-button--short'>
                                                                    <i className='fa fa-ellipsis-v' />
                                                                </button>
                                                            )}
                                                            items={[
                                                                {
                                                                    title: 'Remove',
                                                                    action: () => this.removeKey(gpgkey.keyID)
                                                                }
                                                            ]}
                                                        />
                                                    </div>
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                )) || (
                                    <EmptyState icon='fa fa-key'>
                                        <h4>No GnuPG public keys currently configured</h4>
                                        <h5>You can add GnuPG public keys below..</h5>
                                        <button className='argo-button argo-button--base' onClick={() => (this.showAddGnuPGKey = true)}>
                                            Add GnuPG public key
                                        </button>
                                    </EmptyState>
                                )
                            }
                        </DataLoader>
                    </div>
                </div>
                <SlidingPanel
                    isShown={this.showAddGnuPGKey}
                    onClose={() => (this.showAddGnuPGKey = false)}
                    header={
                        <div>
                            <button className='argo-button argo-button--base' onClick={() => this.formApi.submitForm(null)}>
                                Create
                            </button>{' '}
                            <button onClick={() => (this.showAddGnuPGKey = false)} className='argo-button argo-button--base-o'>
                                Cancel
                            </button>
                        </div>
                    }>
                    <h4>Add GnuPG public key</h4>
                    <Form
                        onSubmit={params => this.addGnuPGPublicKey({keyData: params.keyData})}
                        getApi={api => (this.formApi = api)}
                        preSubmit={(params: NewGnuPGPublicKeyParams) => ({
                            keyData: params.keyData
                        })}
                        validateError={(params: NewGnuPGPublicKeyParams) => ({
                            keyData: !params.keyData && 'Key data is required'
                        })}>
                        {formApi => (
                            <form onSubmit={formApi.submitForm} role='form' className='gpgkeys-list width-control' encType='multipart/form-data'>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='GnuPG public key data (ASCII-armored)' field='keyData' component={TextArea} />
                                </div>
                            </form>
                        )}
                    </Form>
                </SlidingPanel>
            </Page>
        );
    }

    private clearForms() {
        this.formApi.resetAll();
    }

    private validateKeyInputfield(data: string): boolean {
        if (data == null || data === '') {
            return false;
        }
        const str = data.trim();
        const startNeedle = '-----BEGIN PGP PUBLIC KEY BLOCK-----\n';
        const endNeedle = '\n-----END PGP PUBLIC KEY BLOCK-----';

        if (str.length < startNeedle.length + endNeedle.length) {
            return false;
        }
        if (!str.startsWith(startNeedle)) {
            return false;
        }
        if (!str.endsWith(endNeedle)) {
            return false;
        }
        return true;
    }

    private async addGnuPGPublicKey(params: NewGnuPGPublicKeyParams) {
        try {
            if (!this.validateKeyInputfield(params.keyData)) {
                throw {
                    name: 'Invalid key exception',
                    message: 'Invalid GnuPG key data found - must be ASCII armored'
                };
            }
            await services.gpgkeys.create({keyData: params.keyData});
            this.showAddGnuPGKey = false;
            this.loader.reload();
        } catch (e) {
            this.appContext.apis.notifications.show({
                content: <ErrorNotification title='Unable to add GnuPG public key' e={e} />,
                type: NotificationType.Error
            });
        }
    }

    private async removeKey(keyId: string) {
        const confirmed = await this.appContext.apis.popup.confirm('Remove GPG public key', 'Are you sure you want to remove GPG key with ID ' + keyId + '?');
        if (confirmed) {
            await services.gpgkeys.delete(keyId);
            this.loader.reload();
        }
    }

    private get showAddGnuPGKey() {
        return new URLSearchParams(this.props.location.search).get('addGnuPGPublicKey') === 'true';
    }

    private set showAddGnuPGKey(val: boolean) {
        this.clearForms();
        this.appContext.router.history.push(`${this.props.match.url}?addGnuPGPublicKey=${val}`);
    }

    private get appContext(): AppContext {
        return this.context as AppContext;
    }
}
