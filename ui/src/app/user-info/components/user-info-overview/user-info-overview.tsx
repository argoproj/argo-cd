import * as React from 'react';

import {DataLoader, Page} from '../../../shared/components';
import {services} from '../../../shared/services';

require('./user-info-overview.scss');

export const UserInfoOverview = () => (
    <Page title='User Info' toolbar={{breadcrumbs: [{title: 'User Info'}]}}>
        <div className='user-info'>
            <div className='argo-container'>
                <div className='user-info-overview__panel white-box'>
                    <DataLoader key='userInfo' load={() => services.users.get()}>
                        {userInfo =>
                            userInfo.loggedIn ? (
                                <React.Fragment key='userInfoInner'>
                                    <p key='username'>Username: {userInfo.username}</p>
                                    <p key='iss'>Issuer: {userInfo.iss}</p>
                                    {userInfo.groups && (
                                        <React.Fragment key='userInfo4'>
                                            <p>Groups:</p>
                                            <ul>
                                                {userInfo.groups.map(group => (
                                                    <li key={group}>{group}</li>
                                                ))}
                                            </ul>
                                        </React.Fragment>
                                    )}
                                </React.Fragment>
                            ) : (
                                <p key='loggedOutMessage'>You are not logged in</p>
                            )
                        }
                    </DataLoader>
                </div>
            </div>
        </div>
    </Page>
);
