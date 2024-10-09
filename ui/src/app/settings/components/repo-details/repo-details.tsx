import * as React from 'react';
import {FormField} from 'argo-ui';
import {FormApi, Text} from 'react-form';
import {EditablePanel, EditablePanelItem} from '../../../shared/components';
import * as models from '../../../shared/models';
import {NewHTTPSRepoParams} from '../repos-list/repos-list';

export const RepoDetails = (props: {repo: models.Repository; save?: (params: NewHTTPSRepoParams) => Promise<void>}) => {
    const {repo, save} = props;
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
        tlsClientCertData: repo.tlsClientCertData || '',
        tlsClientCertKey: repo.tlsClientCertKey || '',
        insecure: repo.insecure || false,
        enableLfs: repo.enableLfs || false,
        proxy: repo.proxy || '',
        noProxy: repo.noProxy || '',
        project: repo.project || '',
        enableOCI: repo.enableOCI || false,
        forceHttpBasicAuth: repo.forceHttpBasicAuth || false
    };

    return (
        <EditablePanel
            values={repo}
            validate={input => ({
                username: !input.username && input.password && 'Username is required if password is given.',
                password: !input.password && input.username && 'Password is required if username is given.'
            })}
            save={async input => {
                const params: NewHTTPSRepoParams = {...newRepo};
                params.name = input.name || '';
                params.username = input.username || '';
                params.password = input.password || '';
                save(params);
            }}
            title='CONNECTED REPOSITORY'
            items={FormItems(repo)}
        />
    );
};
