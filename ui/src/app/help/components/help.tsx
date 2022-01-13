import * as React from 'react';
import {DataLoader, Page} from '../../shared/components';
import {Consumer} from '../../shared/context';
import {combineLatest} from 'rxjs';
import {services} from '../../shared/services';
import {map} from 'rxjs/operators';

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
                                                <i className='fab fa-linux' /> Linux (amd64)
                                            </a>
                                            &nbsp;
                                            {binaryUrls.hasOwnProperty('linux-arm64') && (
                                                <a href={`${binaryUrls['linux-arm64']}`} className='user-info-panel-buttons argo-button argo-button--base'>
                                                    <i className='fab fa-linux' /> Linux (arm64)
                                                </a>
                                            )}
                                            &nbsp;
                                            {binaryUrls.hasOwnProperty('darwin-amd64') && (
                                                <a href={`${binaryUrls['darwin-amd64']}`} className='user-info-panel-buttons argo-button argo-button--base'>
                                                    <i className='fab fa-apple' /> MacOS (amd64)
                                                </a>
                                            )}
                                            &nbsp;
                                            {binaryUrls.hasOwnProperty('darwin-arm64') && (
                                                <a href={`${binaryUrls['darwin-arm64']}`} className='user-info-panel-buttons argo-button argo-button--base'>
                                                    <i className='fab fa-apple' /> MacOS (arm64)
                                                </a>
                                            )}
                                            &nbsp;
                                            {binaryUrls.hasOwnProperty('windows-amd64') && (
                                                <a href={`${binaryUrls['windows-amd64']}`} className='user-info-panel-buttons argo-button argo-button--base'>
                                                    <i className='fab fa-windows' /> Windows
                                                </a>
                                            )}
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
                        )}
                    </Consumer>
                );
            }}
        </DataLoader>
    );
};
