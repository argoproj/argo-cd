import * as React from 'react';
import {Page} from '../../shared/components';
import {Consumer} from '../../shared/context';

require('./help.scss');

export const Help = () => (
    <Consumer>
        {ctx => (
            <Page title='Help'>
                <div className='row'>
                    <div className='columns large-4 small-6'>
                        <div className='help-box'>
                            <p>New to Argo CD?</p>
                            <a className='argo-button argo-button--base' href='https://argo-cd.readthedocs.io'>
                                Read the docs
                            </a>
                        </div>
                    </div>
                    <div className='columns large-4 small-6'>
                        <div className='help-box'>
                            <p>Want to download the CLI tool?</p>
                            <a href={`download/argocd-linux-amd64`} className='argo-button argo-button--base'>
                                <i className='fab fa-linux ' /> Linux
                            </a>
                            &nbsp;
                            <a href={`download/argocd-darwin-amd64`} className='argo-button argo-button--base'>
                                <i className='fab fa-apple' /> macOS
                            </a>
                            &nbsp;
                            <a href={`download/argocd-windows-amd64.exe`} className='argo-button argo-button--base'>
                                <i className='fab fa-windows' /> Windows
                            </a>
                        </div>
                    </div>
                    <div className='columns large-4 small-6'>
                        <div className='help-box'>
                            <p>You want to develop against Argo CD's API?</p>
                            <a className='argo-button argo-button--base' href='/swagger-ui'>
                                Open the API docs
                            </a>
                        </div>
                    </div>
                </div>
            </Page>
        )}
    </Consumer>
);
