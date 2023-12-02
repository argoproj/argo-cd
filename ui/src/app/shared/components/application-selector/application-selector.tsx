import * as React from 'react';
import {FormFunctionProps} from 'react-form';
import {CheckboxField} from '..';
import * as models from '../../models';
import {
    appInstanceName,
    appQualifiedName,
    AppSetHealthStatusIcon,
    ComparisonStatusIcon,
    HealthStatusIcon,
    isInvokedFromAppsPath,
    OperationPhaseIcon
} from '../../../applications/components/utils';
import {AuthSettingsCtx, ContextApis} from '../../context';
import {History} from 'history';

export const ApplicationSelector = ({
    apps,
    formApi,
    ctx
}: {
    apps: models.AbstractApplication[];
    formApi: FormFunctionProps;
    ctx: ContextApis & {
        history: History<unknown>;
    };
}) => {
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
                        {isInvokedFromAppsPath(ctx.history.location.pathname) && (app as models.Application).isAppOfAppsPattern
                            ? `(App of Apps) ${appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}`
                            : appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                        &nbsp;
                        {isInvokedFromAppsPath(ctx.history.location.pathname) && <ComparisonStatusIcon status={(app as models.Application).status.sync.status} />}
                        &nbsp;
                        {isInvokedFromAppsPath(ctx.history.location.pathname) && <HealthStatusIcon state={(app as models.Application).status.health} />}
                        {!isInvokedFromAppsPath(ctx.history.location.pathname) && <AppSetHealthStatusIcon state={(app as models.ApplicationSet).status} />}
                        &nbsp;
                        {isInvokedFromAppsPath(ctx.history.location.pathname) && <OperationPhaseIcon app={app as models.Application} />}
                        <br />
                    </label>
                ))}
            </div>
        </>
    );
};
