import * as React from 'react';

import {DataLoader, Page} from '../../../shared/components';
import {services} from "../../../shared/services";

export const SessionOverview = () => (
    <Page title='Session' toolbar={{breadcrumbs: [{title: 'Session'}]}}>
        <DataLoader load={() => services.users.get()}>{(session) => (
            <React.Fragment>
                {session.username && <p>Username: {session.username}</p>}
                {session.groups && (<React.Fragment><p>Groups:</p>
                    <ul>
                        {session.groups.map((group) => (<li>{group}</li>))}
                    </ul>
                </React.Fragment>)}
            </React.Fragment>
        )}</DataLoader>
    </Page>
);
