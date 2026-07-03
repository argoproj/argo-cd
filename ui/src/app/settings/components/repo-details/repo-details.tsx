import * as React from 'react';
import {FormField, HelpIcon} from 'argo-ui';
import {FormApi, Text} from 'argo-ui';
import {EditablePanel, EditablePanelItem, NumberField, Repo} from '../../../shared/components';
import {NewHTTPSRepoParams} from '../repos-list/repos-list';
import {AuthSettingsCtx} from '../../../shared/context';
import {UnifiedRepo, getRepoUrl, getRepoName, getRepoType, getRepoProject, getConnectionState, isTemplate, isWrite} from '../repos-list/repos-filter';

export const RepoDetails = (props: {item: UnifiedRepo; save?: (params: NewHTTPSRepoParams) => Promise<void>; readonly?: boolean}) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const {item, save, readonly} = props;

    const template = isTemplate(item);
    const write = isWrite(item);
    const repository = item.readRepo || item.writeRepo;
    const cred = item.readCred || item.writeCred;

    const repoType = getRepoType(item);
    const repoUrl = getRepoUrl(item);
    const repoName = getRepoName(item);
    const project = getRepoProject(item);
    const connectionState = getConnectionState(item);
    const username = repository?.username || cred?.username || '';

    const FormItems = (): EditablePanelItem[] => {
        const items: EditablePanelItem[] = [
            {
                title: 'Type',
                view: repoType
            }
        ];

        if (useAuthSettingsCtx?.hydratorEnabled) {
            items.push({
                title: 'Permission',
                view: write ? 'Write' : 'Read'
            });
        }

        items.push({
            title: template ? 'Credential URL' : 'Repository URL',
            view: <Repo url={repoUrl} />
        });

        if (!template) {
            items.push({
                title: 'Name',
                view: repoName,
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='name' component={Text} />
            });
        }

        if (project) {
            items.push({
                title: 'Project',
                view: project
            });
        }

        items.push(
            {
                title: 'Username (optional)',
                view: username,
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='username' component={Text} />
            },
            {
                title: 'Password (optional)',
                view: username ? '******' : '',
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='password' component={Text} componentProps={{type: 'password'}} />
            }
        );

        if (repoType === 'git') {
            items.push({
                title: 'Bearer token (optional, for BitBucket Data Center only)',
                view: repository?.bearerToken ? '******' : '',
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='bearerToken' component={Text} componentProps={{type: 'password'}} />
            });
        }

        if (repository?.proxy) {
            items.push({
                title: 'Proxy (optional)',
                view: repository.proxy
            });
        }

        if (repository?.noProxy) {
            items.push({
                title: 'NoProxy (optional)',
                view: repository.noProxy
            });
        }

        if (repoType === 'git' && !template) {
            items.push({
                title: 'Depth (optional)',
                view: (repository?.depth || 0).toString(),
                edit: (formApi: FormApi) => (
                    <>
                        <FormField formApi={formApi} field='depth' component={NumberField} />
                        <HelpIcon title='Depth for shallow clones. Leave empty or 0 for a full clone.' />
                    </>
                )
            });
        }

        if (connectionState) {
            items.push({
                title: 'Connection State Details',
                view: connectionState.message
            });
        }

        return items;
    };

    const values = {
        name: repository?.name || repoName,
        username,
        password: '',
        bearerToken: repository?.bearerToken || '',
        depth: repository?.depth || 0
    };

    const baseUpdateParams = repository && {
        type: repository.type,
        name: repository.name || '',
        url: repository.repo,
        username: repository.username || '',
        password: repository.password || '',
        bearerToken: repository.bearerToken || '',
        tlsClientCertData: repository.tlsClientCertData || '',
        tlsClientCertKey: repository.tlsClientCertKey || '',
        insecure: repository.insecure || false,
        enableLfs: repository.enableLfs || false,
        proxy: repository.proxy || '',
        noProxy: repository.noProxy || '',
        project: repository.project || '',
        enableOCI: repository.enableOCI || false,
        forceHttpBasicAuth: repository.forceHttpBasicAuth || false,
        useAzureWorkloadIdentity: repository.useAzureWorkloadIdentity || false,
        insecureOCIForceHttp: repository.insecureOCIForceHttp || false,
        depth: repository.depth || 0
    };

    return (
        <EditablePanel
            values={values}
            validate={input => ({
                username: !input.username && input.password && 'Username is required if password is given.',
                password: !input.password && input.username && 'Password is required if username is given.',
                bearerToken: input.password && input.bearerToken && 'Either the password or the bearer token must be set, but not both.',
                depth: input.depth != undefined && input.depth < 0 && 'Depth must be a non-negative number'
            })}
            save={
                readonly || !save || !baseUpdateParams
                    ? undefined
                    : async input => {
                          const params: NewHTTPSRepoParams = {...baseUpdateParams, write};
                          params.name = input.name || '';
                          params.username = input.username || '';
                          params.password = input.password || '';
                          params.bearerToken = input.bearerToken || '';
                          params.depth = input.depth || 0;
                          await save(params);
                      }
            }
            title={template ? 'CREDENTIAL TEMPLATE' : 'CONNECTED REPOSITORY'}
            items={FormItems()}
        />
    );
};
