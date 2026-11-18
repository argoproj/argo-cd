import * as React from 'react';
import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {FormApi} from 'argo-ui';

import {AutocompleteField, DropDownMenu, FormField} from 'argo-ui';
import {RevisionHelpIcon} from '../../../shared/components';
import {repoQueries} from '../../../shared/queries';
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
    // Every form that renders this field edits an Application, so its project is in the form values.
    const appProject: string | undefined = props.formApi.getFormState().values?.spec?.project;
    const isOci = props.repoType === 'oci';
    const refs = useQuery({...repoQueries.refs(props.repoURL, appProject), enabled: !!props.repoURL && !isOci});
    const ociTags = useQuery({...repoQueries.ociTags(props.repoURL, appProject), enabled: !!props.repoURL && isOci});
    let revisions: string[] = [];
    if (props.repoURL) {
        const available = isOci ? ociTags.data?.tags : selectedFilter === 'Branches' ? refs.data?.branches : refs.data?.tags;
        revisions = ['HEAD'].concat(available || []);
    }
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
