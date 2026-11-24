import {DataLoader, DropDown} from 'argo-ui';
import * as React from 'react';

import {Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {getApplicationSetOwnerRef, getAppListLink} from '../utils';

const MAX_RESULTS = 100; // cap results after filtering to avoid performance issues

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
                            const [apps, appSet] = await Promise.all([
                                services.applications.list([], props.objectListKind, {fields: ['items.metadata.name', 'items.metadata.namespace']}),
                                appSetRef
                                    ? services.applications.getApplicationSet(appSetRef.name, props.application.metadata.namespace).catch((): models.ApplicationSet => null)
                                    : null
                            ]);
                            const accessibleKeys = new Set(apps.items.map(app => `${app.metadata.namespace}/${app.metadata.name}`));
                            const siblings = appSet?.status?.resources?.filter(r => r.kind === 'Application' && accessibleKeys.has(`${r.namespace}/${r.name}`)) ?? [];
                            return {apps, siblings};
                        }}>
                        {({apps, siblings}: {apps: models.AbstractApplicationList; siblings: models.ApplicationSetResource[]}) => {
                            const siblingKeys = new Set(siblings.map(s => `${s.namespace}/${s.name}`));
                            const filteredSiblings =
                                appSetRef && siblings.length > 0
                                    ? siblings.filter(s => appFilter.length === 0 || s.name.toLowerCase().includes(appFilter.toLowerCase())).slice(0, MAX_RESULTS)
                                    : [];
                            return (
                                <>
                                    {filteredSiblings.length > 0 && (
                                        <>
                                            <li className='application-details-app-dropdown__section-header'>IN SET: {appSetRef.name}</li>
                                            {filteredSiblings.map(sibling => {
                                                const siblingApp = {kind: sibling.kind, metadata: {name: sibling.name, namespace: sibling.namespace}} as models.Application;
                                                const siblingLink = getAppListLink(ctx, siblingApp);
                                                return (
                                                    <li className='application-details-app-dropdown__item' key={`sibling-${sibling.name}`}>
                                                        <a href={siblingLink.href} onClick={siblingLink.onClick} className='application-details-app-dropdown__link'>
                                                            <i className='icon argo-icon-application resource-icon__font-icon application-details-app-dropdown__resource-icon' />
                                                            <span>
                                                                {sibling.name}
                                                                {sibling.name === props.appName && ' (current)'}
                                                            </span>
                                                        </a>
                                                    </li>
                                                );
                                            })}
                                            <li className='application-details-app-dropdown__divider' role='separator' />
                                        </>
                                    )}
                                    {apps.items
                                        .filter(app => !siblingKeys.has(`${app.metadata.namespace}/${app.metadata.name}`))
                                        .filter(app => appFilter.length === 0 || app.metadata.name.toLowerCase().includes(appFilter.toLowerCase()))
                                        .slice(0, MAX_RESULTS)
                                        .map(app => {
                                            const appLink = getAppListLink(ctx, app);
                                            return (
                                                <li className='application-details-app-dropdown__item' key={app.metadata.name}>
                                                    <a className='application-details-app-dropdown__link' href={appLink.href} onClick={appLink.onClick}>
                                                        <i
                                                            className={`icon ${resourceIconClass(props.objectListKind)} resource-icon__font-icon application-details-app-dropdown__resource-icon`}
                                                        />
                                                        <span>
                                                            {app.metadata.name}
                                                            {app.metadata.name === props.appName && ' (current)'}
                                                        </span>
                                                    </a>
                                                </li>
                                            );
                                        })}
                                </>
                            );
                        }}
                    </DataLoader>
                </ul>
            )}
        </DropDown>
    );
};
