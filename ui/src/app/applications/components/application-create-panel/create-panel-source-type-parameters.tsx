import {DataLoader, DropDownMenu} from 'argo-ui';
import * as React from 'react';
import {FormApi} from 'react-form';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';
import {ApplicationParameters} from '../application-parameters/application-parameters';
import {APP_SOURCE_TYPES, normalizeTypeFieldsForSource} from '../shared/app-source-edit';

function pathKeyForSource(src: models.ApplicationSource | undefined): string {
    if (!src) {
        return '';
    }
    return (src.chart || src.path || '') as string;
}

export const CreatePanelSourceTypeParameters = (props: {formApi: FormApi; sourceIndex: number}) => {
    const [explicitPathType, setExplicitPathType] = React.useState<{path: string; type: models.AppSourceType}>(null);
    const formApp = props.formApi.getFormState().values as models.Application;
    const src = formApp.spec.sources?.[props.sourceIndex];
    const qeN = props.sourceIndex + 1;

    return (
        <DataLoader
            input={{
                repoURL: src?.repoURL,
                path: src?.path,
                chart: src?.chart,
                targetRevision: src?.targetRevision,
                appName: formApp.metadata.name,
                project: formApp.spec.project,
                pathKey: pathKeyForSource(src)
            }}
            load={async input => {
                if (input.repoURL && input.targetRevision && (input.path || input.chart)) {
                    return services.repos.appDetails(input, input.appName, input.project, 0, 0).catch(() => ({
                        type: 'Directory' as const,
                        details: {}
                    }));
                }
                return {
                    type: 'Directory' as const,
                    details: {}
                };
            }}>
            {(details: models.RepoAppDetails) => {
                const key = pathKeyForSource(src);
                const type = (explicitPathType && explicitPathType.path === key && explicitPathType.type) || details.type;
                let d = details;
                if (d.type !== type) {
                    switch (type) {
                        case 'Helm':
                            d = {
                                type,
                                path: d.path,
                                helm: {name: '', valueFiles: [], path: '', parameters: [], fileParameters: []}
                            };
                            break;
                        case 'Kustomize':
                            d = {type, path: d.path, kustomize: {path: ''}};
                            break;
                        case 'Plugin':
                            d = {type, path: d.path, plugin: {name: '', env: []}};
                            break;
                        default:
                            d = {type, path: d.path, directory: {}};
                            break;
                    }
                }
                return (
                    <React.Fragment>
                        <DropDownMenu
                            anchor={() => (
                                <p>
                                    {type} <i className='fa fa-caret-down' />
                                </p>
                            )}
                            qeId={`application-create-dropdown-source-${qeN}`}
                            items={APP_SOURCE_TYPES.map(item => ({
                                title: item.type,
                                action: () => {
                                    setExplicitPathType({type: item.type, path: key});
                                    normalizeTypeFieldsForSource(props.formApi, item.type, props.sourceIndex);
                                }
                            }))}
                        />
                        <ApplicationParameters
                            noReadonlyMode={true}
                            application={formApp}
                            details={d}
                            multiSourceIndex={props.sourceIndex}
                            save={async updatedApp => {
                                props.formApi.setAllValues(updatedApp);
                            }}
                        />
                    </React.Fragment>
                );
            }}
        </DataLoader>
    );
};
