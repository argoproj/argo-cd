import * as React from 'react';
import {FormFunctionProps} from 'react-form';
import {CheckboxField} from '..';
import * as models from '../../models';
import {appInstanceName, appQualifiedName, AppSetHealthStatusIcon, ComparisonStatusIcon, HealthStatusIcon, isApp, OperationPhaseIcon} from '../../../applications/components/utils';
import {AuthSettingsCtx} from '../../context';

export const ApplicationSelector = ({apps, formApi}: {apps: models.AbstractApplication[]; formApi: FormFunctionProps}) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    return (
        <>
            <label>
                Apps (<a onClick={() => apps.forEach((_, i) => formApi.setValue('app/' + i, true))}>all</a>/
                <a onClick={() => apps.forEach((app, i) => formApi.setValue('app/' + i, (app as models.Application).status.sync.status === models.SyncStatuses.OutOfSync))}>
                    out of sync
                </a>
                /<a onClick={() => apps.forEach((_, i) => formApi.setValue('app/' + i, false))}>none</a>
                ):
            </label>
            <div style={{marginTop: '0.4em'}}>
                {apps.map((app, i) => (
                    <label key={appInstanceName(app)} style={{marginTop: '0.5em', cursor: 'pointer'}}>
                        <CheckboxField field={`app/${i}`} />
                        &nbsp;
                        {/* if not an AppSet, can safely cast to Application. For Appset, (App of Apps) is not needed */}
                        {isApp(app) && (app as models.Application).isAppOfAppsPattern
                            ? `(App of Apps) ${appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}`
                            : appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                        &nbsp;
                        {isApp(app) && <ComparisonStatusIcon status={(app as models.Application).status.sync.status} />}
                        &nbsp;
                        {isApp(app) && <HealthStatusIcon state={(app as models.Application).status.health} />}
                        {!isApp(app) && <AppSetHealthStatusIcon state={(app as models.ApplicationSet).status} />}
                        &nbsp;
                        {isApp(app) && <OperationPhaseIcon app={app as models.Application} />}
                        <br />
                    </label>
                ))}
            </div>
        </>
    );
};
