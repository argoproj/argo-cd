import {DataLoader, DropDown} from 'argo-ui';
import * as React from 'react';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {getAppUrl, getSiblingApplications} from '../utils';

const MAX_RESULTS = 100; // cap results after filtering to avoid performance issues

function resourceIconClass(objectListKind: string): string {
    return objectListKind === 'applicationset' ? 'argo-icon-applicationset' : 'argo-icon-application';
}

export const ApplicationsDetailsAppDropdown = (props: {
    appName: string;
    objectListKind: string;
    application?: models.Application;
    appLabelKey?: string;
    trackingMethod?: string;
}) => {
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
                    <li className='application-details-app-dropdown__divider' role='separator' />
                    <DataLoader
                        load={() =>
                            services.applications.list([], props.objectListKind, {
                                fields: ['items.metadata.name', 'items.metadata.namespace', 'items.metadata.ownerReferences', 'items.metadata.annotations', 'items.metadata.labels']
                            })
                        }>
                        {(apps: models.AbstractApplicationList) => {
                            const group = props.application
                                ? getSiblingApplications(props.application, apps.items as models.Application[], props.appLabelKey, props.trackingMethod)
                                : null;
                            const siblingKeys = new Set((group?.siblings ?? []).map(s => `${s.metadata.namespace}/${s.metadata.name}`));
                            const filteredSiblings = (group?.siblings ?? [])
                                .filter(s => appFilter.length === 0 || s.metadata.name.toLowerCase().includes(appFilter.toLowerCase()))
                                .slice(0, MAX_RESULTS);
                            return (
                                <>
                                    {filteredSiblings.length > 0 && (
                                        <>
                                            <li className='application-details-app-dropdown__section-header'>
                                                {group.parent.kind === 'ApplicationSet' ? 'IN SET' : 'SAME PARENT'}: {group.parent.name}
                                            </li>
                                            {filteredSiblings.map(sibling => (
                                                <li
                                                    className='application-details-app-dropdown__item'
                                                    key={`sibling-${sibling.metadata.namespace}/${sibling.metadata.name}`}
                                                    onClick={() => ctx.navigation.goto(`/${getAppUrl(sibling)}`)}>
                                                    <i className='icon argo-icon-application resource-icon__font-icon application-details-app-dropdown__resource-icon' />
                                                    <span>
                                                        {sibling.metadata.name}
                                                        {sibling.metadata.name === props.appName && ' (current)'}
                                                    </span>
                                                </li>
                                            ))}
                                            <li className='application-details-app-dropdown__divider' role='separator' />
                                        </>
                                    )}
                                    {apps.items
                                        .filter(app => !siblingKeys.has(`${app.metadata.namespace}/${app.metadata.name}`))
                                        .filter(app => appFilter.length === 0 || app.metadata.name.toLowerCase().includes(appFilter.toLowerCase()))
                                        .slice(0, MAX_RESULTS)
                                        .map(app => (
                                            <li
                                                className='application-details-app-dropdown__item'
                                                key={app.metadata.name}
                                                onClick={() => ctx.navigation.goto(`/${getAppUrl(app)}`)}>
                                                <i
                                                    className={`icon ${resourceIconClass(props.objectListKind)} resource-icon__font-icon application-details-app-dropdown__resource-icon`}
                                                />
                                                <span>
                                                    {app.metadata.name}
                                                    {app.metadata.name === props.appName && ' (current)'}
                                                </span>
                                            </li>
                                        ))}
                                </>
                            );
                        }}
                    </DataLoader>
                </ul>
            )}
        </DropDown>
    );
};
