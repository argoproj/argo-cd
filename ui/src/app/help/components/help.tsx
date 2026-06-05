import * as React from 'react';
import {DataLoader, Page} from '../../shared/components';
import {Consumer} from '../../shared/context';
import {services} from '../../shared/services';
import {VersionMessage} from '../../shared/models';
import classNames from 'classnames';

require('./help.scss');

export const Help = () => {
    return (
        <DataLoader
            load={async () => {
                // Return a Promise (not an Observable): DataLoader's observable branch has a
                // completion race that, under React's batched updates, clobbers the loaded value
                // with `undefined` when every source completes (as these promise-backed ones do).
                const [settings, version] = await Promise.all([
                    services.authService.settings(),
                    // Best-effort, informative only: the download link below is arch-agnostic, so the
                    // arch is just label text.
                    services.version.version().catch(() => ({Platform: ''}) as VersionMessage)
                ]);
                return {
                    binaryUrls: settings.help.binaryUrls || {},
                    // Platform is "<os>/<arch>" (e.g. "linux/amd64"); may be empty.
                    hostArch: (version.Platform || '').split('/')[1] || ''
                };
            }}>
            {(settings?: {binaryUrls: Record<string, string>; hostArch: string}) => {
                const binaryUrls = settings?.binaryUrls || {};
                const hostArch = settings?.hostArch || '';
                return (
                    <Consumer>
                        {() => (
                            <Page title='Help'>
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
                                            {/* Arch-agnostic link: the server serves its own embedded binary regardless of
                                                suffix, so this works on any pod and keeps the UI bundle architecture-independent.
                                                The arch is shown for information only, when known. */}
                                            <a href='download/argocd-linux' className='user-info-panel-buttons argo-button argo-button--base'>
                                                <i className='fab fa-linux' /> Linux{hostArch && ` (${hostArch})`}
                                            </a>
                                            &nbsp;
                                            {Object.keys(binaryUrls || {}).map(binaryName => {
                                                const url = binaryUrls[binaryName];
                                                const match = binaryName.match(/.*(darwin|windows|linux)-(amd64|arm64|ppc64le|s390x)/);
                                                const [platform, arch] = match ? match.slice(1) : ['', ''];
                                                return (
                                                    <>
                                                        &nbsp;
                                                        <a key={binaryName} href={url} className='user-info-panel-buttons argo-button argo-button--base'>
                                                            <i
                                                                className={classNames('fab', {
                                                                    'fa-windows': platform === 'windows',
                                                                    'fa-apple': platform === 'darwin',
                                                                    'fa-linux': platform === 'linux'
                                                                })}
                                                            />
                                                            {` ${platform}`} {arch && `(${arch})`}
                                                        </a>
                                                    </>
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
            }}
        </DataLoader>
    );
};
