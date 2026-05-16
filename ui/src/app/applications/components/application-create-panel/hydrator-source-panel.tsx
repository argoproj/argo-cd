import * as React from 'react';
import {FormApi, Text} from 'react-form';
import {AutocompleteField, FormField} from 'argo-ui';

import * as models from '../../../shared/models';
import {RevisionFormField} from '../revision-form-field/revision-form-field';

interface HydratorSourcePanelProps {
    formApi: FormApi;
    repos: string[];
}

interface LabeledRevisionFieldProps {
    formApi: FormApi;
    repoURL: string;
    fieldValue: string;
    repoType?: string;
    revisionType?: 'Branches' | 'Tags';
    helpIconTop?: string;
    compact?: boolean;
}

const LabeledRevisionField = (props: LabeledRevisionFieldProps) => (
    <div style={{display: 'flex', width: '100%'}}>
        <div style={{flex: 1, minWidth: 0}}>
            <RevisionFormField
                formApi={props.formApi}
                helpIconTop={props.helpIconTop}
                repoURL={props.repoURL}
                repoType={props.repoType}
                fieldValue={props.fieldValue}
                revisionType={props.revisionType}
                compact={props.compact !== false}
            />
        </div>
    </div>
);

const subsectionBodyStyle: React.CSSProperties = {
    display: 'flex',
    flexDirection: 'column',
    gap: '2.5rem',
    paddingLeft: '0.75rem',
    borderLeft: '2px solid #e2e5e9'
};

export const HydratorSourcePanel = (props: HydratorSourcePanelProps) => {
    const app = props.formApi.getFormState().values as models.Application;
    const drySourceRepoURL = app.spec.sourceHydrator?.drySource?.repoURL || '';

    return (
        <div style={{display: 'flex', flexDirection: 'column', gap: '1rem'}}>
            <div style={{display: 'flex', flexDirection: 'column'}}>
                <p style={{marginBottom: 0, fontWeight: 600}}>DRY SOURCE</p>
                <div style={subsectionBodyStyle}>
                    <div style={{display: 'flex', width: '100%'}}>
                        <div style={{flex: 1, minWidth: 0}}>
                            <FormField
                                formApi={props.formApi}
                                label='Repository URL'
                                field='spec.sourceHydrator.drySource.repoURL'
                                component={AutocompleteField}
                                componentProps={{
                                    items: props.repos,
                                    filterSuggestions: true
                                }}
                            />
                        </div>
                    </div>
                    <LabeledRevisionField
                        formApi={props.formApi}
                        repoURL={drySourceRepoURL}
                        repoType='git'
                        fieldValue='spec.sourceHydrator.drySource.targetRevision'
                        helpIconTop='2.5em'
                    />
                    <div style={{display: 'flex', width: '100%'}}>
                        <div style={{flex: 1, minWidth: 0}}>
                            <FormField formApi={props.formApi} label='Path' field='spec.sourceHydrator.drySource.path' component={Text} />
                        </div>
                    </div>
                </div>
            </div>
            <div style={{display: 'flex', flexDirection: 'column'}}>
                <p style={{marginBottom: 0, fontWeight: 600}}>SYNC SOURCE</p>
                <div style={subsectionBodyStyle}>
                    <LabeledRevisionField
                        formApi={props.formApi}
                        repoURL={drySourceRepoURL}
                        repoType='git'
                        fieldValue='spec.sourceHydrator.syncSource.targetBranch'
                        revisionType='Branches'
                    />
                    <div style={{display: 'flex', width: '100%'}}>
                        <div style={{flex: 1, minWidth: 0}}>
                            <FormField formApi={props.formApi} label='Path' field='spec.sourceHydrator.syncSource.path' component={Text} />
                        </div>
                    </div>
                </div>
            </div>
            <div style={{display: 'flex', flexDirection: 'column', gap: '0.5rem'}}>
                <p style={{fontWeight: 600}}>HYDRATE TO</p>
                <div style={subsectionBodyStyle}>
                    <LabeledRevisionField
                        formApi={props.formApi}
                        repoURL={drySourceRepoURL}
                        repoType='git'
                        fieldValue='spec.sourceHydrator.hydrateTo.targetBranch'
                        revisionType='Branches'
                    />
                </div>
            </div>
        </div>
    );
};
