import * as React from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {AppearanceList} from './appearance-list';

const mockUpdatePreferences = jest.fn();

jest.mock('argo-ui', () => ({
    Select: ({value, options, onChange}: {value: string; options: {value: string; title: string}[]; onChange: (o: {value: string}) => void}) => {
        const R = require('react');
        return R.createElement(
            'select',
            {value, onChange: (e: React.ChangeEvent<HTMLSelectElement>) => onChange({value: e.target.value})},
            options.map(o => R.createElement('option', {key: o.value, value: o.value}, o.title))
        );
    }
}));
jest.mock('../../../shared/components', () => ({
    Page: ({children}: {children: React.ReactNode}) => children,
    DataLoader: ({children}: {children: (pref: object) => React.ReactNode}) => children({theme: 'auto', groupAppsByProject: false})
}));
jest.mock('../../../shared/services', () => ({
    services: {viewPreferences: {getPreferences: jest.fn(), updatePreferences: (change: object) => mockUpdatePreferences(change)}}
}));

test('offers a Yes/No group by projects selector that updates the preference', () => {
    render(<AppearanceList />);
    expect(screen.getByText('Group by projects')).toBeTruthy();

    const select = screen.getAllByRole('combobox')[1] as HTMLSelectElement;
    expect(select.value).toBe('no');
    expect(Array.from(select.options).map(o => o.text)).toEqual(['Yes', 'No']);

    fireEvent.change(select, {target: {value: 'yes'}});
    expect(mockUpdatePreferences).toHaveBeenCalledWith({groupAppsByProject: true});
});
