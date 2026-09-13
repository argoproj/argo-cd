import * as React from 'react';
import {ReactForm} from 'argo-ui';
import {TagsInput} from './tags-input';

export const TagsInputField = ReactForm.FormField((props: {options: string[]; noTagsLabel?: string; placeholder?: string; fieldApi: ReactForm.FieldApi; className: string}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const tags = (getValue() || []) as Array<string>;

    return (
        <div className='argo-has-value argo-field' style={{border: 'none'}}>
            <TagsInput tags={tags} autocomplete={props.options || []} placeholder={props.placeholder} onChange={vals => setValue(vals)} />
        </div>
    );
});
