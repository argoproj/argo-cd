import * as React from 'react';
import {FormFunctionProps} from 'react-form';
import {CheckboxField} from '..';
import * as models from '../../models';
import {appInstanceName, appQualifiedName, ComparisonStatusIcon, HealthStatusIcon, OperationPhaseIcon} from '../../../applications/components/utils';
import {AuthSettingsCtx} from '../../context';

interface ApplicationSelectorProps {
    apps: models.Application[];
    formApi: FormFunctionProps;
    filterOptions?: {
        showOutOfSync?: boolean;
        showAll?: boolean;
        showNone?: boolean;
    };
}

export const ApplicationSelector = ({
    apps,
    formApi,
    filterOptions = {
        showOutOfSync: true,
        showAll: true,
        showNone: true
    }
}: ApplicationSelectorProps) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    return (
        <>
            <label>
                Apps ({filterOptions?.showAll ? <a onClick={() => apps.forEach((_, i) => formApi.setValue('app/' + i, true))}>all</a> : null}
                {filterOptions?.showOutOfSync ? (
                    <>
                        /<a onClick={() => apps.forEach((app, i) => formApi.setValue('app/' + i, app.status?.sync.status === models.SyncStatuses.OutOfSync))}>out of sync</a>
                    </>
                ) : null}
                {filterOptions?.showNone ? (
                    <>
                        /<a onClick={() => apps.forEach((_, i) => formApi.setValue('app/' + i, false))}>none</a>
                    </>
                ) : null}
                )
            </label>
            <div style={{marginTop: '0.4em'}}>
                {apps.map((app, i) => (
                    <label key={appInstanceName(app)} style={{marginTop: '0.5em', cursor: 'pointer'}}>
                        <CheckboxField field={`app/${i}`} />
                        &nbsp;
                        {app.isAppOfAppsPattern
                            ? `(App of Apps) ${appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}`
                            : appQualifiedName(app, useAuthSettingsCtx?.appsInAnyNamespaceEnabled)}
                        &nbsp;
                        <ComparisonStatusIcon status={app.status.sync.status} />
                        &nbsp;
                        <HealthStatusIcon state={app.status.health} />
                        &nbsp;
                        <OperationPhaseIcon app={app} />
                        <br />
                    </label>
                ))}
            </div>
        </>
    );
};
