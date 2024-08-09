import {DataLoader, DropDown} from 'argo-ui';
import * as React from 'react';

import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';

export const ApplicationsDetailsAppDropdown = (props: {appName: string}) => {
    const [opened, setOpened] = React.useState(false);
    const [appFilter, setAppFilter] = React.useState('');
    const ctx = React.useContext(Context);
    return (
        <DropDown
            onOpenStateChange={setOpened}
            isMenu={true}
            anchor={() => (
                <>
                    <i className='fa fa-search' /> <span>{props.appName}</span>
                </>
            )}>
            {opened && (
                <ul>
                    <li>
                        <input
                            className='argo-field'
                            value={appFilter}
                            onChange={e => setAppFilter(e.target.value)}
                            ref={el =>
                                el &&
                                setTimeout(() => {
                                    if (el) {
                                        el.focus();
                                    }
                                }, 100)
                            }
                        />
                    </li>
                    <DataLoader load={() => services.applications.list([], {fields: ['items.metadata.name', 'items.metadata.namespace']})}>
                        {apps =>
                            apps.items
                                .filter(app => {
                                    return appFilter.length === 0 || app.metadata.name.toLowerCase().includes(appFilter.toLowerCase());
                                })
                                .slice(0, 100) // take top 100 results after filtering to avoid performance issues
                                .map(app => (
                                    <li key={app.metadata.name} onClick={() => ctx.navigation.goto(`/applications/${app.metadata.namespace}/${app.metadata.name}`)}>
                                        {app.metadata.name} {app.metadata.name === props.appName && ' (current)'}
                                    </li>
                                ))
                        }
                    </DataLoader>
                </ul>
            )}
        </DropDown>
    );
};
