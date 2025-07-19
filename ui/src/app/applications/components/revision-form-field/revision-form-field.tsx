import * as React from 'react';
import {useState} from 'react';
import {FormApi} from 'react-form';

import {AutocompleteField, DataLoader, DropDownMenu, FormField} from 'argo-ui';
import {RevisionHelpIcon} from '../../../shared/components';
import {services} from '../../../shared/services';

interface RevisionFormFieldProps {
    formApi: FormApi;
    helpIconTop?: string;
    hideLabel?: boolean;
    repoURL: string;
    fieldValue?: string;
    repoType?: string;
}

export function RevisionFormField({formApi, helpIconTop, hideLabel, repoURL, fieldValue, repoType}: RevisionFormFieldProps) {
    const [filterType, setFilterType] = useState('Branches');

    const extraPadding = hideLabel ? '0em' : '1.53em';
    const rowClass = hideLabel ? '' : ' argo-form-row';

    return (
        <div className={'row' + rowClass}>
            <div className='columns small-10'>
                <DataLoader
                    input={{repoURL}}
                    load={async ({repoURL}: {repoURL: string}): Promise<string[]> => {
                        if (repoType === 'oci' && repoURL) {
                            return services.repos
                                .ociTags(repoURL)
                                .then(res => ['HEAD', ...(res.tags || [])])
                                .catch(() => []);
                        } else if (repoURL) {
                            return services.repos
                                .revisions(repoURL)
                                .then(res => ['HEAD', ...(filterType === 'Branches' ? res.branches || [] : []), ...(filterType === 'Tags' ? res.tags || [] : [])])
                                .catch(() => []);
                        }
                        return [];
                    }}>
                    {(revisions: string[]) => (
                        <FormField
                            formApi={formApi}
                            label={hideLabel ? undefined : 'Revision'}
                            field={fieldValue ?? 'spec.source.targetRevision'}
                            component={AutocompleteField}
                            componentProps={{
                                items: revisions,
                                filterSuggestions: true
                            }}
                        />
                    )}
                </DataLoader>
                <RevisionHelpIcon type='git' top={helpIconTop} right='0em' />
            </div>
            <div style={{paddingTop: extraPadding}} className='columns small-2'>
                {repoType !== 'oci' && (
                    <DropDownMenu
                        anchor={() => (
                            <p>
                                {filterType} <i className='fa fa-caret-down' />
                            </p>
                        )}
                        qeId='application-create-dropdown-revision'
                        items={['Branches', 'Tags'].map(type => ({
                            title: type,
                            action: () => setFilterType(type)
                        }))}
                    />
                )}
            </div>
        </div>
    );
}
