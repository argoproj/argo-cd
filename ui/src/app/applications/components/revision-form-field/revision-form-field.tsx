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

export function RevisionFormField(props: RevisionFormFieldProps) {
    const [filterType, setFilterType] = useState('Branches');

    const setFilter = (newValue: string) => {
        setFilterType(newValue);
    };

    const selectedFilter = filterType;
    const extraPadding = props.hideLabel ? '0em' : '1.53em';
    const rowClass = props.hideLabel ? '' : ' argo-form-row';
    return (
        <div className={'row' + rowClass}>
            <div className='columns small-10'>
                <DataLoader
                    input={{repoURL: props.repoURL, filterType: selectedFilter}}
                    load={async (src: any): Promise<string[]> => {
                        if (props.repoType === 'oci' && src.repoURL) {
                            return services.repos
                                .ociTags(src.repoURL)
                                .then(revisionsRes => ['HEAD'].concat(revisionsRes.tags || []))
                                .catch((): string[] => []);
                        } else if (src.repoURL) {
                            return services.repos
                                .revisions(src.repoURL)
                                .then(revisionsRes =>
                                    ['HEAD']
                                        .concat(selectedFilter === 'Branches' ? revisionsRes.branches || [] : [])
                                        .concat(selectedFilter === 'Tags' ? revisionsRes.tags || [] : [])
                                )
                                .catch((): string[] => []);
                        }
                        return [];
                    }}>
                    {(revisions: string[]) => (
                        <FormField
                            formApi={props.formApi}
                            label={props.hideLabel ? undefined : 'Revision'}
                            field={props.fieldValue ? props.fieldValue : 'spec.source.targetRevision'}
                            component={AutocompleteField}
                            componentProps={{
                                items: revisions,
                                filterSuggestions: true
                            }}
                        />
                    )}
                </DataLoader>
                <RevisionHelpIcon type='git' top={props.helpIconTop} right='0em' />
            </div>
            <div style={{paddingTop: extraPadding}} className='columns small-2'>
                {props.repoType !== 'oci' && (
                    <DropDownMenu
                        anchor={() => (
                            <p>
                                {filterType} <i className='fa fa-caret-down' />
                            </p>
                        )}
                        qeId='application-create-dropdown-revision'
                        items={['Branches', 'Tags'].map((type: 'Branches' | 'Tags') => ({
                            title: type,
                            action: () => {
                                setFilter(type);
                            }
                        }))}
                    />
                )}
            </div>
        </div>
    );
}
