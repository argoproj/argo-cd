import * as React from 'react';
import {FormField} from 'argo-ui';
import {FormApi, Text} from 'react-form';
import {EditablePanel, EditablePanelItem} from '../../../shared/components';
import * as models from '../../../shared/models';
import {NewHTTPSRepoParams} from '../repos-list/repos-list';
import {AuthSettingsCtx} from '../../../shared/context';

export const RepoDetails = (props: {repo: models.Repository; save?: (params: NewHTTPSRepoParams) => Promise<void>; registered?: boolean; applications?: models.Application[]}) => {
    const useAuthSettingsCtx = React.useContext(AuthSettingsCtx);
    const {repo, save, registered = true, applications} = props;
    const write = false;
    const FormItems = (repository: models.Repository): EditablePanelItem[] => {
        const items: EditablePanelItem[] = [
            {
                title: 'Type',
                view: repository.type
            },
            {
                title: 'Repository URL',
                view: repository.repo
            },
            {
                title: 'Name',
                view: repository.name || '',
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='name' component={Text} />
            },
            {
                title: 'Connection State Details',
                view: repository.connectionState.message
            },
            {
                title: 'Username (optional)',
                view: repository.username || '',
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='username' component={Text} />
            },
            {
                title: 'Password (optional)',
                view: repository.username ? '******' : '',
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='password' component={Text} componentProps={{type: 'password'}} />
            }
        ];

        if (applications && applications.length > 0) {
            items.push({
                title: 'Applications',
                view: (
                    <div>
                        {applications.map(app => (
                            <div key={app.metadata.name}>
                                <a href={`/applications/${app.metadata.name}`}>{app.metadata.name}</a>
                            </div>
                        ))}
                    </div>
                )
            });
        }

        if (repository.type === 'git') {
            items.push({
                title: 'Bearer token (optional, for BitBucket Data Center only)',
                view: repository.bearerToken ? '******' : '',
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='bearerToken' component={Text} componentProps={{type: 'password'}} />
            });
        }

        if (useAuthSettingsCtx?.hydratorEnabled) {
            // Insert this item at index 1.
            const item = {
                title: 'Write',
                view: write,
                edit: (formApi: FormApi) => <FormField formApi={formApi} field='write' component={Text} componentProps={{type: 'checkbox'}} />
            };
            items.splice(1, 0, item);
        }

        if (repository.project) {
            items.splice(repository.name ? 2 : 1, 0, {
                title: 'Project',
                view: repository.project
            });
        }

        if (repository.proxy) {
            items.push({
                title: 'Proxy (optional)',
                view: repository.proxy
            });
        }

        if (repository.noProxy) {
            items.push({
                title: 'NoProxy (optional)',
                view: repository.noProxy
            });
        }

        return items;
    };

    const newRepo = {
        type: repo.type,
        name: repo.name || '',
        url: repo.repo,
        username: repo.username || '',
        password: repo.password || '',
        bearerToken: repo.bearerToken || '',
        tlsClientCertData: repo.tlsClientCertData || '',
        tlsClientCertKey: repo.tlsClientCertKey || '',
        insecure: repo.insecure || false,
        enableLfs: repo.enableLfs || false,
        proxy: repo.proxy || '',
        noProxy: repo.noProxy || '',
        project: repo.project || '',
        enableOCI: repo.enableOCI || false,
        forceHttpBasicAuth: repo.forceHttpBasicAuth || false,
        useAzureWorkloadIdentity: repo.useAzureWorkloadIdentity || false,
        insecureOCIForceHttp: repo.insecureOCIForceHttp || false
    };

    return (
        <EditablePanel
            values={repo}
            validate={input => ({
                username: !input.username && input.password && 'Username is required if password is given.',
                password: !input.password && input.username && 'Password is required if username is given.',
                bearerToken: input.password && input.bearerToken && 'Either the password or the bearer token must be set, but not both.'
            })}
            save={
                registered && save
                    ? async input => {
                          const params: NewHTTPSRepoParams = {...newRepo, write};
                          params.name = input.name || '';
                          params.username = input.username || '';
                          params.password = input.password || '';
                          params.bearerToken = input.bearerToken || '';
                          save(params);
                      }
                    : undefined
            }
            title={registered ? 'CONNECTED REPOSITORY' : 'NOT REGISTERED REPOSITORY'}
            items={FormItems(repo)}
        />
    );
};
