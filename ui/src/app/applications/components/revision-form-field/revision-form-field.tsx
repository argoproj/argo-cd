import * as React from 'react';
import {useState} from 'react';
import {FormApi} from 'react-form';

import {AutocompleteField, DataLoader, DropDownMenu, FormField} from 'argo-ui';
import {RevisionHelpIcon} from '../../../shared/components';
import {services} from '../../../shared/services';
import './revision-form-field.scss';

interface RevisionFormFieldProps {
    formApi: FormApi;
    helpIconTop?: string;
    hideLabel?: boolean;
    compact?: boolean;
    repoURL: string;
    fieldValue?: string;
    repoType?: string;
    revisionType?: 'Branches' | 'Tags';
}

export function RevisionFormField(props: RevisionFormFieldProps) {
    const [filterType, setFilterType] = useState('Branches');

    const setFilter = (newValue: string) => {
        setFilterType(newValue);
    };

    const selectedFilter = props.revisionType || filterType;
    const rowClass = props.hideLabel ? '' : ' argo-form-row';
    const rowPaddingRight = !props.revisionType ? '45px' : undefined;
    const wrapperClassName = [props.compact ? '' : 'row' + rowClass, 'revision-form-field'].filter(Boolean).join(' ');
    const wrapperStyle: React.CSSProperties = {
        paddingRight: rowPaddingRight,
        ...(props.compact ? {marginTop: 0, marginBottom: 0} : {})
    };
    return (
        <div className={wrapperClassName} style={wrapperStyle}>
            <div className='revision-form-field__main'>
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
            </div>
            <div className='revision-form-field__dropdown'>
                {props.repoType !== 'oci' && !props.revisionType && (
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
            {!props.revisionType && <RevisionHelpIcon type='git' top={props.helpIconTop} right='0em' />}
        </div>
    );
}
