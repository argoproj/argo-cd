import * as React from 'react';

import {DataLoader, Page} from '../../../shared/components';
import {services} from '../../../shared/services';

require('./user-info-overview.scss');

export const UserInfoOverview = () => (
    <Page title='User Info' toolbar={{breadcrumbs: [{title: 'User Info'}]}}>
        <div className='user-info'>
            <div className='argo-container'>
                <div className='user-info-overview__panel white-box'>
                    <DataLoader key='userInfo' load={() => services.users.get()}>{(userInfo) => (
                        userInfo.loggedIn ? (
                            <React.Fragment>
                                <p>Username: {userInfo.username}</p>
                                <p>Issuer: {userInfo.iss}</p>
                                {userInfo.groups && (<React.Fragment><p>Groups:</p>
                                    <ul>
                                        {userInfo.groups.map((group) => (
                                            <li>{group}</li>
                                        ))}
                                    </ul>
                                </React.Fragment>)}
                            </React.Fragment>
                        ) : (
                            <p>You are not logged in</p>
                        )
                    )}</DataLoader>
                </div>
            </div>
        </div>
    </Page>
);
