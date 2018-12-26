import { Autocomplete } from 'argo-ui';
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
                            <div ref={(el) => {
                                if (el && !el.onkeyup) {
                                    el.onkeyup = (keyEvent) => keyEvent.keyCode === 13 && api.submitForm(null);
                                }
                            }}>
                                <Autocomplete
                                    options={props.options}
                                    inputProps={{className: props.className, style: { borderBottom: 'none' }}}
                                    wrapperProps={{className: props.className}} value={api.values.path} onChange={(val, e) => {
                                        if (e.selected) {
                                            setValue((getValue() || []).concat(val));
                                        } else {
                                            api.setValue('path', val);
                                        }
                                    }}/> <i className='fa fa-plus' onClick={() => api.submitForm(null)}/>
                            </div>
                        )}
                    </ReactForm.Form>
                </div>
            </div>
        </div>
    );
});
