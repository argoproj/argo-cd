import {Checkbox} from 'argo-ui';
import * as React from 'react';
import * as ReactForm from 'react-form';

export const SetFinalizerOnApplication = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const finalizerval = 'resources-finalizer.argocd.argoproj.io';
    const setval = getValue() || [];
    const isChecked = setval.indexOf(finalizerval) === -1 ? false : true;
    const val = [finalizerval] || [];
    return (
        <div className='argo-field' style={{borderBottom: '0'}}>
            <React.Fragment>
                <Checkbox
                    id='set-finalizer'
                    checked={isChecked}
                    onChange={() => {
                        if (!isChecked) {
                            setValue(val);
                        } else {
                            setValue([]);
                        }
                    }}
                />
                <label htmlFor={`set-finalizer`}>Set Deletion Finalizer</label>
            </React.Fragment>
        </div>
    );
});
