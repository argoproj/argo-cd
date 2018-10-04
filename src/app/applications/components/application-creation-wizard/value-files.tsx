import { FormAutocomplete, FormField } from 'argo-ui';
import * as classNames from 'classnames';
import * as React from 'react';
import * as ReactForm from 'react-form';

export const ValueFiles = ReactForm.FormField((props: {paths: string[], fieldApi: ReactForm.FieldApi, className: string }) => {
    const { fieldApi: {getValue, setValue}} = props;
    const paths = (getValue() || []) as Array<string>;

    return (
        <div className={classNames(props.className, { 'application-creation-wizard__values-files': true, 'argo-has-value': true })}>
            {paths.map((path, i) => (
                <span title={path} className='application-creation-wizard__values-file-label' key={path}><i className='fa fa-times'
                onClick={() => {
                    const newPaths = paths.slice();
                    newPaths.splice(i, 1);
                    setValue(newPaths);
                }}/> {path}</span>
            ))}
            <div className='application-creation-wizard__values-files-autocomplete'>
                <ReactForm.Form onSubmit={(vals, _, api) => {
                    setValue(paths.concat(vals.path));
                    api.resetAll();
                }} validateError={(vals) => ({ path: !vals.path && 'Path or URL is required.' })}>
                    {(api) => (
                        <React.Fragment>
                            <FormField field='path' formApi={api} componentProps={{options: props.paths}} component={FormAutocomplete}/> <i className='fa fa-plus'
                                onClick={() => api.submitForm(null)}/>
                        </React.Fragment>
                    )}
                </ReactForm.Form>
            </div>
        </div>
    );
});
