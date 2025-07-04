import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Key, KeybindingContext, NumKey, NumKeyToNumber, NumPadKey, useNav} from 'argo-ui/v2';
import {Cluster} from '../../../shared/components';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../../../applications/components/utils';
import {services} from '../../../shared/services';
import {ResourceIcon} from '../../../applications/components/resource-icon';

import './resources-tiles.scss';

export interface ResourceTilesProps {
    resources: models.Resource[];
}

const useItemsPerContainer = (itemRef: any, containerRef: any): number => {
    const [itemsPer, setItemsPer] = React.useState(0);

    React.useEffect(() => {
        const handleResize = () => {
            let timeoutId: any;
            clearTimeout(timeoutId);
            timeoutId = setTimeout(() => {
                timeoutId = null;
                const itemWidth = itemRef.current ? itemRef.current.offsetWidth : -1;
                const containerWidth = containerRef.current ? containerRef.current.offsetWidth : -1;
                const curItemsPer = containerWidth > 0 && itemWidth > 0 ? Math.floor(containerWidth / itemWidth) : 1;
                if (curItemsPer !== itemsPer) {
                    setItemsPer(curItemsPer);
                }
            }, 1000);
        };
        window.addEventListener('resize', handleResize);
        handleResize();
        return () => {
            window.removeEventListener('resize', handleResize);
        };
    }, []);

    return itemsPer || 1;
};

export const ResourceTiles = ({resources}: ResourceTilesProps) => {
    const [selectedApp, navApp, reset] = useNav(resources.length);

    const ctxh = React.useContext(Context);
    const appRef = {ref: React.useRef(null), set: false};
    const appContainerRef = React.useRef(null);
    const appsPerRow = useItemsPerContainer(appRef.ref, appContainerRef);

    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding({keys: Key.RIGHT, action: () => navApp(1)});
    useKeybinding({keys: Key.LEFT, action: () => navApp(-1)});
    useKeybinding({keys: Key.DOWN, action: () => navApp(appsPerRow)});
    useKeybinding({keys: Key.UP, action: () => navApp(-1 * appsPerRow)});

    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(
                    AppUtils.getAppUrl({
                        metadata: {
                            name: resources[selectedApp].appName,
                            namespace: resources[selectedApp].namespace
                        }
                    } as models.Application)
                );
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            if (selectedApp > -1) {
                reset();
                return true;
            }
            return false;
        }
    });

    useKeybinding({
        keys: Object.values(NumKey) as NumKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    useKeybinding({
        keys: Object.values(NumPadKey) as NumPadKey[],
        action: n => {
            reset();
            return navApp(NumKeyToNumber(n));
        }
    });
    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => {
                        return (
                            <div className='resources-tiles argo-table-list argo-table-list--clickable' ref={appContainerRef}>
                                {resources.map((app, i) => {
                                    return (
                                        <div
                                            key={`${app.appProject}/${app.appName}/${app.name}/${app.namespace}/${app.name}/${app.kind}/${app.group}/${app.version}`}
                                            ref={appRef.set ? null : appRef.ref}
                                            className={`argo-table-list__row resources-list__entry resources-list__entry--health-${app.health?.status} ${
                                                selectedApp === i ? 'resources-tiles__selected' : ''
                                            }`}>
                                            <div
                                                className='row resources-tiles__wrapper'
                                                onClick={e =>
                                                    ctx.navigation.goto(
                                                        `${AppUtils.getAppUrl({
                                                            metadata: {
                                                                name: app.appName,
                                                                namespace: app.appNamespace
                                                            }
                                                        } as models.Application)}`,
                                                        {
                                                            view: pref.appDetails.view,
                                                            node: `/${app.kind}${app.namespace ? `/${app.namespace}` : ''}/${app.name}/0`
                                                        },
                                                        {event: e}
                                                    )
                                                }>
                                                <div className={`columns small-12 resources-list__info resources-tiles__item`}>
                                                    <div className='row '>
                                                        <div className='columns small-3'>
                                                            <ResourceIcon
                                                                kind={app.kind}
                                                                customStyle={{
                                                                    padding: '2px',
                                                                    lineHeight: '40px',
                                                                    width: '40px',
                                                                    height: '40px'
                                                                }}
                                                            />
                                                        </div>
                                                        <div className='columns small-9'>
                                                            {app.group}/{app.version}
                                                        </div>
                                                    </div>
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Kind'>
                                                            Name:
                                                        </div>
                                                        <div className='columns small-9'>{app.name}</div>
                                                    </div>
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Kind'>
                                                            Kind:
                                                        </div>
                                                        <div className='columns small-9'>{app.kind}</div>
                                                    </div>
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Project:'>
                                                            Project:
                                                        </div>
                                                        <div className='columns small-9'>{app.appProject}</div>
                                                    </div>
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Project:'>
                                                            Application:
                                                        </div>
                                                        <div className='columns small-9'>
                                                            <a
                                                                onClick={e =>
                                                                    ctx.navigation.goto(
                                                                        AppUtils.getAppUrl({
                                                                            metadata: {
                                                                                name: app.appName,
                                                                                namespace: app.appNamespace
                                                                            }
                                                                        } as models.Application),
                                                                        {view: pref.appDetails.view},
                                                                        {event: e}
                                                                    )
                                                                }
                                                                target='_blank'
                                                                style={{fontSize: '16px', fontWeight: 400}}>
                                                                {app.appName}
                                                            </a>
                                                        </div>
                                                    </div>
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Status:'>
                                                            Status:
                                                        </div>

                                                        <div className='columns small-9' qe-id='resources-tiles-health-status'>
                                                            {app?.health && (
                                                                <>
                                                                    <AppUtils.HealthStatusIcon state={app?.health} /> {app?.health?.status}
                                                                </>
                                                            )}{' '}
                                                            &nbsp;
                                                            <AppUtils.ComparisonStatusIcon status={app?.status} /> {app?.status}
                                                            &nbsp;
                                                        </div>
                                                    </div>
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Destination:'>
                                                            Cluster:
                                                        </div>
                                                        <div className='columns small-9'>
                                                            <Cluster server={app.clusterServer} name={app.clusterName} />
                                                        </div>
                                                    </div>
                                                    <div className='row'>
                                                        <div className='columns small-3' title='Namespace:'>
                                                            Namespace:
                                                        </div>
                                                        <div className='columns small-9'>{app.namespace}</div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        );
                    }}
                </DataLoader>
            )}
        </Consumer>
    );
};
