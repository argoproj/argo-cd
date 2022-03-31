import {FormField} from 'argo-ui';
import * as React from 'react';
import * as ReactForm from 'react-form';
import {Form, Text} from 'react-form';

interface ProjectRoleGroupsProps {
    projName: string;
    roleName: string;
    groups: string[];
    formApi: ReactForm.FormApi;
    newRole: boolean;
}

export const ProjectRoleGroupsEdit = (props: ProjectRoleGroupsProps) => (
    <React.Fragment>
        <p>GROUPS</p>
        <div>OIDC group names to bind to this role</div>
        {
            <div className='argo-table-list'>
                <div className='argo-table-list__row'>
                    {(props.groups || []).map((groupName, i) => (
                        <Group
                            key={i}
                            field={['groups', i]}
                            formApi={props.formApi}
                            projName={props.projName}
                            roleName={props.roleName}
                            groupName={groupName}
                            deleteGroup={() => props.formApi.setValue('groups', removeEl(props.groups, i))}
                        />
                    ))}
                </div>
            </div>
        }

        <Form>
            {api => (
                <div className='argo-table-list'>
                    <div className='argo-table-list__row'>
                        <div className='row'>
                            <div className='columns small-8'>
                                <FormField formApi={api} label='' field='groupName' component={Text} />
                            </div>
                            <div className='columns small-4'>
                                <a
                                    className='argo-button argo-button--base'
                                    onClick={() => {
                                        if (api.values.groupName.length > 0) {
                                            props.formApi.setValue('groups', (props.formApi.values.groups || []).concat(api.values.groupName));
                                            api.values.groupName = '';
                                        }
                                    }}>
                                    Add group
                                </a>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </Form>
    </React.Fragment>
);

interface GroupProps {
    projName: string;
    roleName: string;
    groupName: string;
    fieldApi: ReactForm.FieldApi;
    deleteGroup: () => void;
}

function removeEl(items: any[], index: number) {
    items.splice(index, 1);
    return items;
}

class GroupWrapper extends React.Component<GroupProps, any> {
    public render() {
        return (
            <div className='row'>
                <div className='columns small-11'>{this.props.groupName}</div>
                <div className='columns small-1'>
                    <i className='fa fa-times' onClick={() => this.props.deleteGroup()} style={{cursor: 'pointer'}} />
                </div>
            </div>
        );
    }
}

const Group = ReactForm.FormField(GroupWrapper);
