import * as React from 'react';
import {useQuery} from '@tanstack/react-query';
import {Page} from '../../shared/components';
import {Consumer} from '../../shared/context';
import {helpQueries} from '../../shared/queries';
import {loading, loadFailed} from '../../shared/query-client';
import classNames from 'classnames';

require('./help.scss');

export const Help = () => {
    const settings = useQuery(helpQueries.settings());
    const version = useQuery(helpQueries.version());
    if (!settings.data) {
        return settings.isFetching || !settings.isError ? loading : loadFailed(settings.refetch);
    }
    const binaryUrls = settings.data.help.binaryUrls || {};
    const hostArch = (version.data?.Platform || '').split('/')[1] || '';
    return (
        <Consumer>
            {() => (
                <Page title='Help'>
                    {settings.isRefetchError && !settings.isFetching && loadFailed(settings.refetch)}
                    <div className='row'>
                        <div className='columns large-4 small-6'>
                            <div className='help-box'>
                                <p>New to Argo CD?</p>
                                <a className='user-info-panel-buttons argo-button argo-button--base' href='https://argo-cd.readthedocs.io'>
                                    Read the docs
                                </a>
                            </div>
                        </div>
                        <div className='columns large-4 small-6'>
                            <div className='help-box'>
                                <p>Want to download the CLI tool?</p>
                                {/* Arch-agnostic link: targets the suffix-less /argocd-linux route, which every
                                                server serves from its own embedded binary regardless of the server's architecture.
                                                This keeps the UI bundle architecture-independent (no arch baked in) and avoids
                                                404s on mixed-arch clusters. The arch label is informational only, shown when known. */}
                                <a href='download/argocd-linux' className='user-info-panel-buttons argo-button argo-button--base'>
                                    <i className='fab fa-linux' /> Linux{hostArch && ` (${hostArch})`}
                                </a>
                                &nbsp;
                                {Object.keys(binaryUrls || {}).map(binaryName => {
                                    const url = binaryUrls[binaryName];
                                    const match = binaryName.match(/.*(darwin|windows|linux)-(amd64|arm64|ppc64le|s390x)/);
                                    const [platform, arch] = match ? match.slice(1) : ['', ''];
                                    return (
                                        <React.Fragment key={binaryName}>
                                            &nbsp;
                                            <a href={url} className='user-info-panel-buttons argo-button argo-button--base'>
                                                <i
                                                    className={classNames('fab', {
                                                        'fa-windows': platform === 'windows',
                                                        'fa-apple': platform === 'darwin',
                                                        'fa-linux': platform === 'linux'
                                                    })}
                                                />
                                                {` ${platform}`} {arch && `(${arch})`}
                                            </a>
                                        </React.Fragment>
                                    );
                                })}
                            </div>
                        </div>
                        <div className='columns large-4 small-6'>
                            <div className='help-box'>
                                <p>You want to develop against Argo CD's API?</p>
                                <a className='user-info-panel-buttons argo-button argo-button--base' href='swagger-ui'>
                                    Open the API docs
                                </a>
                            </div>
                        </div>
                    </div>
                </Page>
            )}
        </Consumer>
    );
};
