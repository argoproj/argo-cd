import * as React from 'react';
import * as ReactForm from 'react-form';
import {TagsInput} from './tags-input';

export const TagsInputField = ReactForm.FormField((props: {options: string[]; noTagsLabel?: string; fieldApi: ReactForm.FieldApi; className: string}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const tags = (getValue() || []) as Array<string>;

    return (
        <div className='argo-has-value argo-field' style={{border: 'none'}}>
            <TagsInput tags={tags} autocomplete={props.options || []} onChange={vals => setValue(vals)} />
        </div>
    );
});
