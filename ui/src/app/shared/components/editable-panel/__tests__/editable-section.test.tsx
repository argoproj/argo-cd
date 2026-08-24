import * as React from 'react';
import {render, screen, waitFor} from '@testing-library/react';
import {act} from 'react';
import {FormApi} from 'argo-ui';

jest.mock('../../../../applications/components/utils', () => ({
    helpTip: (text: string) => React.createElement('span', {title: text}, '?')
}));

import {EditableSection} from '../editable-section';
import {EditablePanelItem} from '../editable-panel';

const ctx = {
    notifications: {
        show: jest.fn()
    }
} as any;

describe('EditableSection – noReadonlyMode', () => {
    test('saves a user change once and does not save the parent echo', async () => {
        const save = jest.fn().mockResolvedValue(undefined);
        let capturedFormApi: FormApi | null = null;
        const items: EditablePanelItem[] = [
            {
                title: 'Path',
                view: <span />,
                edit: (api: FormApi) => {
                    capturedFormApi = api;
                    return <span data-testid='form-value'>{api.values.path}</span>;
                }
            }
        ];

        const {rerender} = render(
            <EditableSection uniqueId='source' values={{path: 'apps'}} items={items} save={save} noReadonlyMode ctx={ctx} />
        );

        await waitFor(() => expect(capturedFormApi).not.toBeNull());
        save.mockClear();

        await act(async () => {
            capturedFormApi?.setValue('path', 'apps/demo');
        });

        await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
        expect(save).toHaveBeenLastCalledWith({path: 'apps/demo'}, {});

        rerender(<EditableSection uniqueId='source' values={{path: 'apps/demo'}} items={items} save={save} noReadonlyMode ctx={ctx} />);

        await waitFor(() => expect(screen.getByTestId('form-value')).toHaveTextContent('apps/demo'));
        expect(save).toHaveBeenCalledTimes(1);
    });

    test('does not save values synchronized from the parent', async () => {
        const save = jest.fn().mockResolvedValue(undefined);
        const items: EditablePanelItem[] = [
            {
                title: 'Path',
                view: <span />,
                edit: (api: FormApi) => <span data-testid='form-value'>{api.values.path}</span>
            }
        ];

        const {rerender} = render(<EditableSection uniqueId='source' values={{path: 'v1'}} items={items} save={save} noReadonlyMode ctx={ctx} />);

        await waitFor(() => expect(screen.getByTestId('form-value')).toHaveTextContent('v1'));
        save.mockClear();

        rerender(<EditableSection uniqueId='source' values={{path: 'v2'}} items={items} save={save} noReadonlyMode ctx={ctx} />);

        await waitFor(() => expect(screen.getByTestId('form-value')).toHaveTextContent('v2'));
        expect(save).not.toHaveBeenCalled();
    });
});
