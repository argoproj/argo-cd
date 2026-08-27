import * as React from 'react';
import {render, screen, fireEvent} from '@testing-library/react';
import {Form, FormApi} from 'argo-ui';

import {ImageTagFieldEditor} from './kustomize';

function renderEditor(metadataValue: string, defaultValues: any = {}) {
    let api: FormApi | null = null;
    render(
        <Form
            defaultValues={defaultValues}
            getApi={a => {
                api = a;
            }}>
            {() => <ImageTagFieldEditor field='images[0]' metadata={{value: metadataValue}} className='' />}
        </Form>
    );
    return () => (api as unknown as FormApi).values.images[0];
}

test('editing tag of renamed image keeps original name as matcher', () => {
    const getValue = renderEditor('my-image=ghcr.io/my-org/my-image:main');
    fireEvent.change(screen.getByPlaceholderText('main'), {target: {value: 'abc123'}});
    expect(getValue()).toBe('my-image=ghcr.io/my-org/my-image:abc123');
});

test('editing tag of plain image produces tag-only override', () => {
    const getValue = renderEditor('nginx:1.15.4');
    fireEvent.change(screen.getByPlaceholderText('1.15.4'), {target: {value: '1.16.0'}});
    expect(getValue()).toBe('nginx:1.16.0');
});

test('editing tag of existing override keeps its new name', () => {
    const getValue = renderEditor('my-image=ghcr.io/my-org/my-image:main', {images: ['my-image=ghcr.io/my-org/my-image:v1']});
    fireEvent.change(screen.getByDisplayValue('v1'), {target: {value: 'v2'}});
    expect(getValue()).toBe('my-image=ghcr.io/my-org/my-image:v2');
});
