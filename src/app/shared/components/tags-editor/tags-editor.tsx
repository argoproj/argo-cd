import { FormAutocomplete, FormField } from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import * as ReactForm from 'react-form';

require('./tags-editor.scss');

export const TagsEditor = ReactForm.FormField((props: {options: string[], noTagsLabel?: string, fieldApi: ReactForm.FieldApi, className: string }) => {
    const { fieldApi: {getValue, setValue}} = props;
    const tags = (getValue() || []) as Array<string>;

    return (
        <div className={classNames(props.className, { 'tags-editor': true, 'argo-has-value': true })}>
            {tags.length > 0 && (
                <div className='tags-editor__tags'>
                    {tags.map((path, i) => (
                        <span title={path} className='tags-editor__tag' key={i}><i className='fa fa-times'
                        onClick={() => {
                            const newPaths = tags.slice();
                            newPaths.splice(i, 1);
                            setValue(newPaths);
                        }}/> {path}</span>
                    ))}
                </div>
            ) || <p>{props.noTagsLabel || 'No tags added'}</p>}
            <div className='tags-editor__autocomplete'>
                <div className='argo-field'>
                    <ReactForm.Form onSubmit={(vals, _, api) => {
                        setValue(tags.concat(vals.path));
                        api.resetAll();
                    }} validateError={(vals) => ({ path: !vals.path && 'Value is required.' })}>
                        {(api) => (
                            <React.Fragment>
                                <FormField field='path' formApi={api} componentProps={{options: props.options}} component={FormAutocomplete}/> <i className='fa fa-plus'
                                    onClick={() => api.submitForm(null)}/>
                            </React.Fragment>
                        )}
                    </ReactForm.Form>
                </div>
            </div>
        </div>
    );
});
