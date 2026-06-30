import {DataLoader, DropDown} from 'argo-ui';
import * as React from 'react';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {getAppUrl, getApplicationSetOwnerRef} from '../utils';

function resourceIconClass(objectListKind: string): string {
    return objectListKind === 'applicationset' ? 'argo-icon-applicationset' : 'argo-icon-application';
}

export const ApplicationsDetailsAppDropdown = (props: {appName: string; objectListKind: string; application?: models.Application}) => {
    const [opened, setOpened] = React.useState(false);
    const [appFilter, setAppFilter] = React.useState('');
    const ctx = React.useContext(Context);

    const appSetRef = props.application ? getApplicationSetOwnerRef(props.application) : null;

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
                    <DataLoader
                        load={async () => {
                            const [appsResult, appSetResult] = await Promise.allSettled([
                                services.applications.list([], props.objectListKind, {fields: ['items.metadata.name', 'items.metadata.namespace']}),
                                appSetRef
                                    ? services.applications.getApplicationSet(appSetRef.name, props.application.metadata.namespace)
                                    : Promise.reject(new Error('no appset'))
                            ]);
                            const apps = appsResult.status === 'fulfilled' ? appsResult.value : {items: []};
                            const appSet = appSetResult.status === 'fulfilled' ? appSetResult.value : null;
                            const siblings = appSet?.status?.resources?.filter(r => r.kind === 'Application') ?? [];
                            return {apps, siblings};
                        }}>
                        {({apps, siblings}: {apps: models.AbstractApplicationList; siblings: models.ApplicationSetResource[]}) => {
                            const filteredSiblings =
                                appSetRef && siblings.length > 0
                                    ? siblings.filter(s => appFilter.length === 0 || s.name.toLowerCase().includes(appFilter.toLowerCase()))
                                    : [];
                            return (
                                <>
                                    {filteredSiblings.length > 0 && (
                                        <>
                                            <li className='application-details-app-dropdown__section-header'>IN SET: {appSetRef.name}</li>
                                            {filteredSiblings.map(sibling => (
                                                <li
                                                    className='application-details-app-dropdown__item'
                                                    key={`sibling-${sibling.name}`}
                                                    onClick={() => ctx.navigation.goto(`/applications/${sibling.namespace}/${sibling.name}`)}>
                                                    <i className='icon argo-icon-application resource-icon__font-icon application-details-app-dropdown__resource-icon' />
                                                    <span>
                                                        {sibling.name}
                                                        {sibling.name === props.appName && ' (current)'}
                                                    </span>
                                                </li>
                                            ))}
                                            <li className='application-details-app-dropdown__divider' role='separator' />
                                        </>
                                    )}
                                    {apps.items
                                        .filter(app => appFilter.length === 0 || app.metadata.name.toLowerCase().includes(appFilter.toLowerCase()))
                                        .slice(0, 100)
                                        .map(app => (
                                            <li
                                                className='application-details-app-dropdown__item'
                                                key={app.metadata.name}
                                                onClick={() => ctx.navigation.goto(`/${getAppUrl(app)}`)}>
                                                <i className={`icon ${resourceIconClass(props.objectListKind)} resource-icon__font-icon application-details-app-dropdown__resource-icon`} />
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
