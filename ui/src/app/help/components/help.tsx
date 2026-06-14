import * as React from 'react';
import {DataLoader, Page} from '../../shared/components';
import {Consumer} from '../../shared/context';
import {services} from '../../shared/services';
import {VersionMessage} from '../../shared/models';
import classNames from 'classnames';

require('./help.scss');

export const RELEASES_URL = 'https://github.com/argoproj/argo-cd/releases';

// Released versions ("v3.4.1", "v3.4.0-rc1") link to their release tag; dev builds
// ("v3.5.0+0dc5b3f") and unknown versions fall back to the releases list.
export const cliDownloadURL = (version: string): string => {
    const tag = (version || '').trim();
    return /^v\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$/.test(tag) ? `${RELEASES_URL}/tag/${tag}` : RELEASES_URL;
};

export const Help = () => {
    return (
        <DataLoader
            load={async () => {
                // Return a Promise (not an Observable): DataLoader's observable branch has a
                // completion race that, under React's batched updates, clobbers the loaded value
                // with `undefined` when every source completes (as these promise-backed ones do).
                const [settings, version] = await Promise.all([
                    services.authService.settings(),
                    // Best-effort: an unknown version just links to the releases list.
                    services.version.version().catch(() => ({Version: ''}) as VersionMessage)
                ]);
                return {
                    binaryUrls: settings.help.binaryUrls || {},
                    downloadURL: cliDownloadURL(version.Version)
                };
            }}>
            {(settings?: {binaryUrls: Record<string, string>; downloadURL: string}) => {
                const binaryUrls = settings?.binaryUrls || {};
                const downloadURL = settings?.downloadURL || RELEASES_URL;
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
                                            <a href={downloadURL} target='_blank' rel='noopener noreferrer' className='user-info-panel-buttons argo-button argo-button--base'>
                                                <i className='fab fa-github' /> GitHub releases
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
