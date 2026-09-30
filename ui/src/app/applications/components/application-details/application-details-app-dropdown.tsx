import {DataLoader, DropDown} from 'argo-ui';
import * as React from 'react';

import {Context} from '../../../shared/context';
import {services} from '../../../shared/services';
import {getAppUrl} from '../utils';

function resourceIconClass(objectListKind: string): string {
    return objectListKind === 'applicationset' ? 'argo-icon-applicationset' : 'argo-icon-application';
}

export const ApplicationsDetailsAppDropdown = (props: {appName: string; appNamespace: string; objectListKind: string}) => {
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
                    <li className='application-details-app-dropdown__filter'>
                        <span className='application-details-app-dropdown__filter-spacer' aria-hidden='true' />
                        <input
                            className='argo-field'
                            value={appFilter}
                            onChange={e => setAppFilter(e.target.value)}
                            ref={el => {
                                if (el) {
                                    setTimeout(() => {
                                        if (el) {
                                            el.focus();
                                        }
                                    }, 100);
                                }
                            }}
                        />
                    </li>
                    <DataLoader load={() => services.applications.list([], props.objectListKind, {fields: ['items.metadata.name', 'items.metadata.namespace']})}>
                        {apps => {
                            const filtered = apps.items
                                .filter(app => {
                                    return appFilter.length === 0 || app.metadata.name.toLowerCase().includes(appFilter.toLowerCase());
                                })
                                .slice(0, 100); // take top 100 results after filtering to avoid performance issues

                            // Determine which names appear in more than one namespace so we can show
                            // the namespace prefix only when disambiguation is needed.
                            const nameCounts = new Map<string, number>();
                            for (const app of filtered) {
                                nameCounts.set(app.metadata.name, (nameCounts.get(app.metadata.name) ?? 0) + 1);
                            }

                            return filtered.map(app => {
                                const isCurrent = app.metadata.name === props.appName && app.metadata.namespace === props.appNamespace;
                                const needsNamespace = app.metadata.namespace && nameCounts.get(app.metadata.name) > 1;
                                const label = needsNamespace ? `${app.metadata.namespace}/${app.metadata.name}` : app.metadata.name;
                                const key = app.metadata.namespace ? `${app.metadata.namespace}/${app.metadata.name}` : app.metadata.name;
                                return (
                                    <li className='application-details-app-dropdown__item' key={key} onClick={() => ctx.navigation.goto(`/${getAppUrl(app)}`)}>
                                        <i className={`icon ${resourceIconClass(props.objectListKind)} resource-icon__font-icon application-details-app-dropdown__resource-icon`} />
                                        <span>
                                            {label}
                                            {isCurrent && ' (current)'}
                                        </span>
                                    </li>
                                );
                            });
                        }}
                    </DataLoader>
                </ul>
            )}
        </DropDown>
    );
};
