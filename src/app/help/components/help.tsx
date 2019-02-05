import * as React from 'react';

import { Page } from '../../shared/components';

require('./help.scss');

export const Help = () => (
    <Page title='Help'>
        <div className='row'>
            <div className='columns large-4 small-12'>
                <div className='help-box'>
                    <div className='help-box__ico help-box__ico--manual'/>
                    <h3>Documentation</h3>
                    <a href='https://github.com/argoproj/argo-cd/tree/master/docs' target='_blank' className='help-box__link'>Argo CD Docs</a>
                    <a href='/swagger-ui' target='_blank' className='help-box__link'>Swagger UI</a>
                </div>
            </div>
            <div className='columns large-4 small-12'>
                <div className='help-box'>
                    <div className='help-box__ico help-box__ico--email'/>
                    <h3>Contact</h3>
                    <a className='help-box__link' target='_blank' href='https://groups.google.com/forum/#!forum/argoproj'>Argo Community</a>
                    <a className='help-box__link' target='_blank' href='http://argoproj.io/community/join-slack'>Slack Channel</a>
                </div>
            </div>
            <div className='columns large-4 small-12'>
                <div className='help-box'>
                    <div className='help-box__ico help-box__ico--download'/>
                    <h3>Argo CD CLI</h3>
                    <div className='row text-left help-box__download'>
                        <div className='columns small-6'>
                            <a href={`download/argocd-linux-amd64`}><i
                                    className='fa fa-linux' aria-hidden='true'/> Linux
                            </a>
                        </div>
                        <div className='columns small-6'>
                            <a href={`download/argocd-darwin-amd64`}><i
                                    className='fa fa-apple' aria-hidden='true'/> macOS
                            </a><br/>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </Page>
);
