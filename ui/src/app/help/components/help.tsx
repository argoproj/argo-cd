import * as React from 'react';
import {Page} from '../../shared/components';
import {combineLatest} from 'rxjs';
import {services} from '../../shared/services';
import {map} from 'rxjs/operators';
import classNames from 'classnames';

require('./help.scss');

export const Help = () => {
    const downloadButtons = combineLatest([services.authService.settings()]).pipe(
        map(items => {
            Object.keys(items[0].help.binaryUrls || {}).map(binaryName => {
                const binaryUrls = items[0].help.binaryUrls;
                const url = binaryUrls[binaryName];
                const match = binaryName.match(/.*(darwin|windows|linux)-(amd64|arm64)/);
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
                            />{' '}
                            platform {arch && `( ${arch} )`}
                        </a>
                    </>
                );
            });
        })
    );
    return (
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
                            <i className='fab fa-linux' /> Linux
                        </a>
                        {downloadButtons}
                    </div>
                </div>
                <div className='columns large-4 small-6'>
                    <div className='help-box'>
                        <p>You want to develop against Argo CD's API?</p>
                        <a className='user-info-panel-buttons argo-button argo-button--base' href='/swagger-ui'>
                            Open the API docs
                        </a>
                    </div>
                </div>
            </div>
        </Page>
    );
};
