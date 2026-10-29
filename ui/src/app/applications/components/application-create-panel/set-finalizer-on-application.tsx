import {Checkbox, HelpIcon} from 'argo-ui';
import * as React from 'react';
import {ReactForm} from 'argo-ui';

export const SetFinalizerOnApplication = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const finalizerVal = 'resources-finalizer.argocd.argoproj.io';
    const isDeletionFinalizer = (item: string) => item === finalizerVal || item === 'argocd.argoproj.io/resources-finalizer';
    const currentValue = getValue() || [];
    const index = currentValue.findIndex(isDeletionFinalizer);
    const isChecked = index < 0 ? false : true;
    return (
        <div className='small-12 large-6' style={{borderBottom: '0'}}>
            <Checkbox
                id='set-finalizer'
                checked={isChecked}
                onChange={(state: boolean) => {
                    const value = getValue() || [];
                    if (!state) {
                        setValue(value.filter((item: string) => !isDeletionFinalizer(item)));
                    } else {
                        const tmp = value.slice();
                        tmp.push(finalizerVal);
                        setValue(tmp);
                    }
                }}
            />
            <label htmlFor={`set-finalizer`}>Set Deletion Finalizer</label>
            <HelpIcon title='If checked, the resources deletion finalizer will be set on the application. Potentially destructive, refer to the documentation for more information on the effects of the finalizer.' />
        </div>
    );
});
