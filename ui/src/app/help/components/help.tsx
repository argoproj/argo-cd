import * as React from 'react';
import {t} from 'i18next';
import {DataLoader, Page} from '../../shared/components';
import {Consumer} from '../../shared/context';
import {combineLatest} from 'rxjs';
import {services} from '../../shared/services';
import {map} from 'rxjs/operators';
import classNames from 'classnames';
import en from '../../locales/en';

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
                                            <p>{t('help.new-to-argo-cd.title', en['help.new-to-argo-cd.title'])}</p>
                                            <a className='user-info-panel-buttons argo-button argo-button--base' href='https://argo-cd.readthedocs.io'>
                                                {t('help.new-to-argo-cd.button', en['help.new-to-argo-cd.button'])}
                                            </a>
                                        </div>
                                    </div>
                                    <div className='columns large-4 small-6'>
                                        <div className='help-box'>
                                            <p>{t('help.want-to-download-cli.title', en['help.want-to-download-cli.title'])}</p>
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
                                            <p>{t('help.api.title', en['help.api.title'])}</p>
                                            <a className='user-info-panel-buttons argo-button argo-button--base' href='/swagger-ui'>
                                                {t('help.api.button', en['help.api.button'])}
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
