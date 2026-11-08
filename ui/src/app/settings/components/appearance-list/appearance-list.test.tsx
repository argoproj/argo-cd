import * as React from 'react';
import {act, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';

import {services} from '../../../shared/services';
import {AppearanceList} from './appearance-list';

jest.mock('../../../shared/components', () => ({
    ...jest.requireActual('../../../shared/components'),
    Page: (props: {children?: React.ReactNode}) => props.children
}));

jest.mock('../../../shared/services', () => {
    const {ViewPreferencesService} = jest.requireActual('../../../shared/services/view-preferences-service');
    return {services: {viewPreferences: new ViewPreferencesService()}};
});

describe('AppearanceList', () => {
    beforeEach(() => {
        window.localStorage.clear();
        services.viewPreferences.init();
        services.viewPreferences.updatePreferences({theme: 'light'});
    });

    it('renders the stored theme without a loading state', () => {
        const {container} = render(<AppearanceList />);

        expect(container.querySelector('.select__value')).toHaveTextContent('Light');
        expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    });

    it('follows theme changes made elsewhere', () => {
        const {container} = render(<AppearanceList />);

        act(() => services.viewPreferences.updatePreferences({theme: 'dark'}));

        expect(container.querySelector('.select__value')).toHaveTextContent('Dark');
    });
});
