import * as React from 'react';
import {FormApi} from 'react-form';

import {AutocompleteField, DataLoader, DropDownMenu, FormField} from 'argo-ui';
import {RevisionHelpIcon} from '../../../shared/components';
import {services} from '../../../shared/services';

interface RevisionFormFieldProps {
    formApi: FormApi;
    helpIconTop?: string;
    hideLabel?: boolean;
    repoURL: string;
}

export class RevisionFormField extends React.PureComponent<RevisionFormFieldProps, {filterType: string}> {
    constructor(props: RevisionFormFieldProps) {
        super(props);
        this.state = {filterType: 'Branches'};
    }

    public setFilter(newValue: string) {
        this.setState({filterType: newValue});
    }

    public render() {
        const selectedFilter = this.state.filterType;
        const extraPadding = this.props.hideLabel ? '0em' : '1.53em';
        const rowClass = this.props.hideLabel ? '' : ' argo-form-row';
        return (
            <div className={'row' + rowClass}>
                <div className='columns small-10'>
                    <React.Fragment>
                        <DataLoader
                            input={{repoURL: this.props.repoURL, filterType: selectedFilter}}
                            load={async (src: any): Promise<string[]> => {
                                if (src.repoURL) {
                                    return services.repos
                                        .revisions(src.repoURL)
                                        .then(revisionsRes =>
                                            ['HEAD']
                                                .concat(selectedFilter === 'Branches' ? revisionsRes.branches || [] : [])
                                                .concat(selectedFilter === 'Tags' ? revisionsRes.tags || [] : [])
                                        )
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
                        <RevisionHelpIcon type='git' top={this.props.helpIconTop} right='0em' />
                    </React.Fragment>
                </div>
                <div style={{paddingTop: extraPadding}} className='columns small-2'>
                    <DropDownMenu
                        anchor={() => (
                            <p>
                                {this.state.filterType} <i className='fa fa-caret-down' />
                            </p>
                        )}
                        qeId='application-create-dropdown-revision'
                        items={['Branches', 'Tags'].map((type: 'Branches' | 'Tags') => ({
                            title: type,
                            action: () => {
                                this.setFilter(type);
                            }
                        }))}
                    />
                </div>
            </div>
        );
    }
}
