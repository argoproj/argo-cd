import {Checkbox, HelpIcon} from 'argo-ui';
import * as React from 'react';
import * as ReactForm from 'react-form';

export const SetFinalizerOnApplication = ReactForm.FormField((props: {fieldApi: ReactForm.FieldApi}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const finalizerval = 'resources-finalizer.argocd.argoproj.io';
    const setval = getValue() || [];
    const index = setval.findIndex((item: string) => item === finalizerval);
    const isChecked = index < 0 ? false : true;
    return (
        <div className='small-12 large-6' style={{borderBottom: '0'}}>
            <React.Fragment>
                <Checkbox
                    id='set-finalizer'
                    checked={isChecked}
                    onChange={(state: boolean) => {
                        if (!state) {
                            const v = getValue() || [];
                            const index2 = v.findIndex((item: string) => item === finalizerval);
                            if (index2 >= 0) {
                                const u = v.slice();
                                u.splice(index2, 1);
                                setValue(u);
                            }
                        } else {
                            const v = getValue() || [];
                            const c = v.slice();
                            c.push(finalizerval);
                            setValue(c);
                        }
                    }}
                />
                <label htmlFor={`set-finalizer`}>Set Deletion Finalizer</label>
                <HelpIcon title='If checked finalizer resources-finalizer.argocd.argoproj.io will be set and application resources deletion will be cascaded' />
            </React.Fragment>
        </div>
    );
});
