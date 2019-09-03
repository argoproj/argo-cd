import * as React from 'react';

import {DataLoader, Page} from '../../../shared/components';
import {services} from "../../../shared/services";

require('./session-overview.scss');

export const SessionOverview = () => (
    <Page title='Session' toolbar={{breadcrumbs: [{title: 'Session'}]}}>
        <div className='session-overview'>
            <div className='argo-container'>
                <div className='session-overview__panel white-box'>
                    <DataLoader load={() => services.users.get()}>{(session) => (
                        session.loggedIn ? (
                            <React.Fragment>
                                <p>Username: {session.username}</p>
                                <p>Groups:</p>
                                <ul>
                                    {session.groups.map((group) => (
                                        <li>{group}</li>
                                    ))}
                                </ul>
                            </React.Fragment>
                        ) : (
                            <p>You are logged in</p>
                        )
                    )}</DataLoader>
                </div>
            </div>
        </div>
    </Page>
);
