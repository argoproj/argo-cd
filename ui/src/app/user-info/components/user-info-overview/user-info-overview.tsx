import * as React from 'react';

import {DataLoader, Page} from '../../../shared/components';
import {services} from '../../../shared/services';

require('./user-info-overview.scss');

export const UserInfoOverview = () => (
    <Page title='User Info' toolbar={{breadcrumbs: [{title: 'User Info'}]}}>
        <div className='user-info'>
            <div className='argo-container'>
                <div className='user-info-overview__panel white-box'>
                    <h4>User Info</h4>
                    <DataLoader key='userInfo' load={() => services.users.get()}>{(userInfo) => (
                        userInfo.loggedIn ? (
                            <React.Fragment key='userInfoInner'>
                                <p key='username'>Username: {userInfo.username}</p>
                                <p key='iss'>Issuer: {userInfo.iss}</p>
                                {userInfo.groups && (<React.Fragment key='userInfo4'><p>Groups:</p>
                                    <ul>
                                        {userInfo.groups.map((group) => (
                                            <li key={group}>{group}</li>
                                        ))}
                                    </ul>
                                </React.Fragment>)}
                            </React.Fragment>
                        ) : (
                            <p key='loggedOutMessage'>You are not logged in</p>
                        )
                    )}</DataLoader>
                    <h4>Permissions</h4>
                    {[
                        {action: 'sync', resource: 'applications'},
                        {action: 'create', resource: 'applications'},
                        {action: 'delete', resource: 'applications'},
                        {action: 'update', resource: 'projects'},
                        {action: 'update', resource: 'clusters'},
                    ].map(({action, resource}) => (
                        <p>
                            <DataLoader key={action}
                                        loadingRenderer={() => <React.Fragment><i className='fa fa-question-circle'/>
                                        </React.Fragment>}
                                        errorRenderer={(e) => <React.Fragment><i className='fa fa-exclamation-circle'/>
                                        </React.Fragment>}
                                        load={() => services.users.canI(action, resource, '*')}>{(canI: { value: string }) => (
                                <React.Fragment>{canI.value === 'yes' ? <i
                                    className='fa fa-check-circle'/> : <i
                                    className='fa fa-times-circle'/>} </React.Fragment>)}</DataLoader>
                            {action} {resource}
                        </p>
                    ))}
                </div>
            </div>
        </div>
    </Page>
);
