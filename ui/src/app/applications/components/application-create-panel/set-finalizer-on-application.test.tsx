import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {Form, FormApi} from 'argo-ui';

import {SetFinalizerOnApplication} from './set-finalizer-on-application';

const qualifiedFinalizer = 'argocd.argoproj.io/resources-finalizer';
const existingFinalizer = 'resources-finalizer.argocd.argoproj.io';

jest.mock('argo-ui', () => ({...jest.requireActual('argo-ui'), HelpIcon: () => null}));

function renderField(finalizers?: string[]) {
    let formApi: FormApi;
    render(
        <Form defaultValues={{finalizers}} getApi={api => (formApi = api)}>
            {() => <SetFinalizerOnApplication field='finalizers' />}
        </Form>
    );
    return () => formApi.values.finalizers;
}

test('enabling cascading deletion keeps the existing finalizer name', () => {
    const values = renderField(['example.com/keep']);
    fireEvent.click(screen.getByLabelText('Set Deletion Finalizer'));
    expect(values()).toEqual(['example.com/keep', existingFinalizer]);
});

test('enabling cascading deletion initializes an unset finalizer list', () => {
    const values = renderField();
    fireEvent.click(screen.getByLabelText('Set Deletion Finalizer'));
    expect(values()).toEqual([existingFinalizer]);
});

test.each([
    {name: 'qualified', finalizers: [qualifiedFinalizer]},
    {name: 'existing', finalizers: [existingFinalizer]},
    {name: 'mixed', finalizers: [qualifiedFinalizer, existingFinalizer]}
])('disabling cascading deletion removes $name finalizers', ({finalizers}) => {
    const values = renderField(['example.com/keep', ...finalizers]);
    const checkbox = screen.getByLabelText('Set Deletion Finalizer');
    expect(checkbox).toBeChecked();
    fireEvent.click(checkbox);
    expect(values()).toEqual(['example.com/keep']);
});
