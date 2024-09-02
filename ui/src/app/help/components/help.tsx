import * as React from 'react';
import {DataLoader, Page} from '../../shared/components';
import {Consumer} from '../../shared/context';
import {combineLatest} from 'rxjs';
import {services} from '../../shared/services';
import {map} from 'rxjs/operators';
import classNames from 'classnames';

require('./help.scss');

export const Help = () => {
    return (
        <DataLoader
            load={() =>
                combineLatest([services.authService.settings()]).pipe(
                    map(items => {
                        return {
                            binaryUrls: items[0].help.binaryUrls || {}
                        };
                    })
                )
            }>
            {({binaryUrls}: {binaryUrls: Record<string, string>}) => {
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
                                            <a href={`download/argocd-linux-${process.env.HOST_ARCH}`} className='user-info-panel-buttons argo-button argo-button--base'>
                                                <i className='fab fa-linux' /> Linux ({process.env.HOST_ARCH})
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
