import * as React from 'react';
import {FormApi} from 'react-form';

import {AutocompleteField, DataLoader, FormField} from 'argo-ui';
import {RevisionHelpIcon} from '../../../shared/components';
import {services} from '../../../shared/services';

interface RevisionFormFieldProps {
    formApi: FormApi;
    helpIconTop?: string;
    hideLabel?: boolean;
    repoURL: string;
}

export class RevisionFormField extends React.PureComponent<RevisionFormFieldProps> {
    public render() {
        return (
            <React.Fragment>
                <DataLoader
                    input={{repoURL: this.props.repoURL}}
                    load={async (src: any): Promise<string[]> => {
                        if (src.repoURL) {
                            return services.repos
                                .revisions(src.repoURL)
                                .then(revisionsRes => ['HEAD'].concat(revisionsRes.branches || []).concat(revisionsRes.tags || []))
                                .catch(() => []);
                        }
                        return [];
                    }}>
                    {(revisions: string[]) => (
                        <FormField
                            formApi={this.props.formApi}
                            label={this.props.hideLabel ? undefined : 'Revision'}
                            field='spec.source.targetRevision'
                            component={AutocompleteField}
                            componentProps={{
                                items: revisions,
                                filterSuggestions: true
                            }}
                        />
                    )}
                </DataLoader>
                <RevisionHelpIcon type='git' top={this.props.helpIconTop} />
            </React.Fragment>
        );
    }
}
