import {DropDownMenu, FormField, NotificationType, SlidingPanel} from 'argo-ui';
import React, {useRef, useContext} from 'react';
import {Form, FormApi, TextArea} from 'react-form';
import {withRouter, RouteComponentProps} from 'react-router-dom';

import {DataLoader, EmptyState, ErrorNotification, Page} from '../../../shared/components';
import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

require('./gpgkeys-list.scss');

interface NewGnuPGPublicKeyParams {
    keyData: string;
}

export const GpgKeysList = ({match, location}: RouteComponentProps) => {
    const ctx = useContext(Context);

    const formApi = useRef<FormApi>();
    const loader = useRef<DataLoader>();

    const clearForms = () => {
        formApi.current.resetAll();
    };

    const validateKeyInputfield = (data: string): boolean => {
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
    };

    const addGnuPGPublicKey = async (params: NewGnuPGPublicKeyParams) => {
        try {
            if (!validateKeyInputfield(params.keyData)) {
                throw {
                    name: 'Invalid key exception',
                    message: 'Invalid GnuPG key data found - must be ASCII armored'
                };
            }
            await services.gpgkeys.create({keyData: params.keyData});
            setAddGnuPGKey(false);
            loader.current.reload();
        } catch (e) {
            ctx.notifications.show({
                content: <ErrorNotification title='Unable to add GnuPG public key' e={e} />,
                type: NotificationType.Error
            });
        }
    };

    const removeKey = async (keyId: string) => {
        const confirmed = await ctx.popup.confirm('Remove GPG public key', 'Are you sure you want to remove GPG key with ID ' + keyId + '?');
        if (confirmed) {
            await services.gpgkeys.delete(keyId);
            loader.current.reload();
        }
    };

    const showAddGnuPGKey = () => {
        return new URLSearchParams(location.search).get('addGnuPGPublicKey') === 'true';
    };

    const setAddGnuPGKey = (val: boolean) => {
        clearForms();
        ctx.history.push(`${match.url}?addGnuPGPublicKey=${val}`);
    };

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
                            iconClassName: 'fa fa-plus',
                            action: () => setAddGnuPGKey(true)
                        }
                    ]
                }
            }}>
            <div className='gpgkeys-list'>
                <div className='argo-container'>
                    <DataLoader load={() => services.gpgkeys.list()} ref={ref => (loader.current = ref)}>
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
                                                                action: () => removeKey(gpgkey.keyID)
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
                                    <h5>You can add GnuPG public keys below.</h5>
                                    <button className='argo-button argo-button--base' onClick={() => setAddGnuPGKey(true)}>
                                        Add GnuPG public key
                                    </button>
                                </EmptyState>
                            )
                        }
                    </DataLoader>
                </div>
            </div>
            <SlidingPanel
                isShown={showAddGnuPGKey()}
                onClose={() => setAddGnuPGKey(false)}
                header={
                    <div>
                        <button className='argo-button argo-button--base' onClick={() => formApi.current.submitForm(null)}>
                            Create
                        </button>{' '}
                        <button onClick={() => setAddGnuPGKey(false)} className='argo-button argo-button--base-o'>
                            Cancel
                        </button>
                    </div>
                }>
                <Form
                    onSubmit={params => addGnuPGPublicKey({keyData: params.keyData})}
                    getApi={api => (formApi.current = api)}
                    preSubmit={(params: NewGnuPGPublicKeyParams) => ({
                        keyData: params.keyData
                    })}
                    validateError={(params: NewGnuPGPublicKeyParams) => ({
                        keyData: !params.keyData && 'GnuPG public key data is required'
                    })}>
                    {formApi => (
                        <form onSubmit={formApi.submitForm} role='form' className='gpgkeys-list width-control' encType='multipart/form-data'>
                            <div className='white-box'>
                                <p>ADD GnuPG PUBLIC KEY</p>
                                <div className='argo-form-row'>
                                    <FormField formApi={formApi} label='GnuPG public key data (ASCII-armored)' field='keyData' component={TextArea} />
                                </div>
                            </div>
                        </form>
                    )}
                </Form>
            </SlidingPanel>
        </Page>
    );
};

export default withRouter(GpgKeysList);
