import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {Form, FormApi} from 'argo-ui';
import * as models from '../../models';
import {AuthSettingsCtx} from '../../context';
import {ApplicationSelector} from './application-selector';

jest.mock('react-virtualized/dist/commonjs/AutoSizer', () => ({
    __esModule: true,
    default: ({children}: {children: (size: {width: number; height: number}) => React.ReactNode}) => children({width: 800, height: 600})
}));

jest.mock('react-virtualized/dist/commonjs/List', () => ({
    __esModule: true,
    default: ({rowCount, rowRenderer}: {rowCount: number; rowRenderer: (props: {index: number; key: string; style: React.CSSProperties}) => React.ReactNode}) => (
        <div data-testid='virtualized-list' data-count={rowCount}>
            {[0, 1, rowCount - 1].map(index => rowRenderer({index, key: String(index), style: {}}))}
        </div>
    )
}));

it('updates individual and bulk selections once while preserving other form values and app names', () => {
    const apps = [
        {
            metadata: {name: 'parent', namespace: 'team'},
            status: {health: {status: models.HealthStatuses.Healthy}, sync: {status: models.SyncStatuses.OutOfSync}},
            isAppOfAppsPattern: true
        },
        {metadata: {name: 'synced', namespace: 'team'}, status: {health: {status: models.HealthStatuses.Healthy}, sync: {status: models.SyncStatuses.Synced}}},
        {metadata: {name: 'unknown', namespace: 'team'}, status: {health: {status: models.HealthStatuses.Healthy}}}
    ] as models.Application[];
    let api: FormApi;
    const setValue = jest.fn((field, value) => api.setValue(field, value));
    const setAllValues = jest.fn(values => api.setAllValues(values));
    render(
        <AuthSettingsCtx.Provider value={{appsInAnyNamespaceEnabled: true} as models.AuthSettings}>
            <Form defaultValues={{syncFlags: {prune: true}}} getApi={value => (api = value)}>
                {formApi => <ApplicationSelector apps={apps} formApi={{...formApi, setValue, setAllValues}} />}
            </Form>
        </AuthSettingsCtx.Provider>
    );
    const checkboxes = screen.getAllByRole('checkbox') as HTMLInputElement[];
    expect(screen.getByLabelText('(App of Apps) team/parent')).toBe(checkboxes[0]);
    fireEvent.click(checkboxes[0]);
    expect(checkboxes[0].checked).toBe(true);
    expect(setValue).toHaveBeenCalledTimes(1);
    expect(setValue).toHaveBeenCalledWith('app/0', true);

    fireEvent.click(screen.getByText('all'));
    expect(setAllValues).toHaveBeenCalledTimes(1);
    expect(setAllValues).toHaveBeenLastCalledWith({'syncFlags': {prune: true}, 'app/0': true, 'app/1': true, 'app/2': true});
    expect(checkboxes.map(checkbox => checkbox.checked)).toEqual([true, true, true]);

    fireEvent.click(screen.getByText('out of sync'));
    expect(setAllValues).toHaveBeenCalledTimes(2);
    expect(checkboxes.map(checkbox => checkbox.checked)).toEqual([true, false, false]);

    fireEvent.click(screen.getByText('none'));
    expect(setAllValues).toHaveBeenCalledTimes(3);
    expect(checkboxes.map(checkbox => checkbox.checked)).toEqual([false, false, false]);
    expect(api.values).toEqual({'syncFlags': {prune: true}, 'app/0': false, 'app/1': false, 'app/2': false});
    expect(setValue).toHaveBeenCalledTimes(1);
});

it('selects unmounted apps in one update and uses the real index for virtualized rows', () => {
    const apps = Array.from({length: 60}, (_, i) => ({
        metadata: {name: `app-${i}`},
        status: {health: {status: models.HealthStatuses.Healthy}, sync: {status: models.SyncStatuses.Synced}}
    })) as models.Application[];
    let api: FormApi;
    const setValue = jest.fn((field, value) => api.setValue(field, value));
    const setAllValues = jest.fn(values => api.setAllValues(values));
    render(<Form getApi={value => (api = value)}>{formApi => <ApplicationSelector apps={apps} formApi={{...formApi, setValue, setAllValues}} />}</Form>);
    expect(screen.getByTestId('virtualized-list').getAttribute('data-count')).toBe('60');
    expect(screen.getAllByRole('checkbox')).toHaveLength(3);
    fireEvent.click(screen.getByText('all'));
    expect(setAllValues).toHaveBeenCalledTimes(1);
    expect(api.values).toEqual(Object.fromEntries(apps.map((_, i) => [`app/${i}`, true])));
    fireEvent.click(screen.getByLabelText('app-59'));
    expect(setValue).toHaveBeenCalledTimes(1);
    expect(setValue).toHaveBeenCalledWith('app/59', false);
});

it('keeps the shortcuts available for an empty app list', () => {
    let api: FormApi;
    const setValue = jest.fn((field, value) => api.setValue(field, value));
    const setAllValues = jest.fn(values => api.setAllValues(values));
    render(
        <Form defaultValues={{syncFlags: {prune: true}}} getApi={value => (api = value)}>
            {formApi => <ApplicationSelector apps={[]} formApi={{...formApi, setValue, setAllValues}} />}
        </Form>
    );
    expect(screen.getByText('out of sync')).toBeTruthy();
    expect(screen.getByText('none')).toBeTruthy();
    fireEvent.click(screen.getByText('all'));
    expect(setAllValues).toHaveBeenCalledTimes(1);
    expect(setAllValues).toHaveBeenCalledWith({syncFlags: {prune: true}});
});
