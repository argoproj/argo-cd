import * as models from '../shared/models';
import { State } from './state';

export const ACTION_TYPES = {
    APPLICATIONS_LOAD_REQUEST: 'APPLICATIONS_LOAD_REQUEST',
    APPLICATIONS_LOAD_SUCCESS: 'APPLICATIONS_LOAD_SUCCESS',
    APPLICATIONS_CHANGED: 'APPLICATIONS_CHANGED',

    APPLICATION_LOAD_REQUEST: 'APPLICATION_LOAD_REQUEST',
    APPLICATION_LOAD_SUCCESS: 'APPLICATION_LOAD_SUCCESS',
};

export default function(state: State = { }, action: any): State {
    switch (action.type) {
        case ACTION_TYPES.APPLICATIONS_LOAD_REQUEST:
            return {...state, applications: null, changesSubscription: action.changesSubscription };
        case ACTION_TYPES.APPLICATIONS_LOAD_SUCCESS:
            return {...state, applications: action.applications, changesSubscription: action.changesSubscription};
        case ACTION_TYPES.APPLICATION_LOAD_REQUEST:
            return {...state, application: null, changesSubscription: action.changesSubscription };
        case ACTION_TYPES.APPLICATION_LOAD_SUCCESS:
            return {...state, application: action.application, changesSubscription: action.changesSubscription};
        case ACTION_TYPES.APPLICATIONS_CHANGED:
            const applicationChange: models.ApplicationWatchEvent = action.applicationChange;
            switch (applicationChange.type) {
                case 'ADDED':
                case 'MODIFIED':
                    const index = state.applications.findIndex((item) => item.metadata.name === applicationChange.application.metadata.name);
                    if (index > -1) {
                        return {...state, applications: [...state.applications.slice(0, index), applicationChange.application, ...state.applications.slice(index + 1)]};
                    } else {
                        return {...state, applications: [applicationChange.application, ...state.applications] };
                    }
                case 'DELETED':
                    return {...state, applications: state.applications.filter((item) => item.metadata.name !== applicationChange.application.metadata.name) };
            }
            break;
    }
    return state;
}
