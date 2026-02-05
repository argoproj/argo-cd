import {DataLoader} from 'argo-ui';
import * as React from 'react';
import {Key, KeybindingContext, useNav} from 'argo-ui/v2';
import {Consumer, Context} from '../../../shared/context';
import * as models from '../../../shared/models';
import * as AppUtils from '../utils';
import {isApp} from '../utils';
import {services} from '../../../shared/services';
import {ApplicationTableRow} from './application-table-row';
import {AppSetTableRow} from './appset-table-row';

import './applications-table.scss';

export const ApplicationsTable = (props: {
    applications: models.AbstractApplication[];
    syncApplication: (appName: string, appNamespace: string) => any;
    refreshApplication: (appName: string, appNamespace: string) => any;
    deleteApplication: (appName: string, appNamespace: string) => any;
}) => {
    const [selectedApp, navApp, reset] = useNav(props.applications.length);
    const ctxh = React.useContext(Context);

    const {useKeybinding} = React.useContext(KeybindingContext);

    useKeybinding({keys: Key.DOWN, action: () => navApp(1)});
    useKeybinding({keys: Key.UP, action: () => navApp(-1)});
    useKeybinding({
        keys: Key.ESCAPE,
        action: () => {
            reset();
            return selectedApp > -1 ? true : false;
        }
    });
    useKeybinding({
        keys: Key.ENTER,
        action: () => {
            if (selectedApp > -1) {
                ctxh.navigation.goto(`/${AppUtils.getAppUrl(props.applications[selectedApp])}`);
                return true;
            }
            return false;
        }
    });

    return (
        <Consumer>
            {ctx => (
                <DataLoader load={() => services.viewPreferences.getPreferences()}>
                    {pref => (
                        <div className='applications-table argo-table-list argo-table-list--clickable'>
                            {props.applications.map((app, i) =>
                                isApp(app) ? (
                                    <ApplicationTableRow
                                        key={AppUtils.appInstanceName(app)}
                                        app={app as models.Application}
                                        selected={selectedApp === i}
                                        pref={pref}
                                        ctx={ctx}
                                        syncApplication={props.syncApplication}
                                        refreshApplication={props.refreshApplication}
                                        deleteApplication={props.deleteApplication}
                                    />
                                ) : (
                                    <AppSetTableRow key={AppUtils.appInstanceName(app)} appSet={app as models.ApplicationSet} selected={selectedApp === i} pref={pref} ctx={ctx} />
                                )
                            )}
                        </div>
                    )}
                </DataLoader>
            )}
        </Consumer>
    );
};
