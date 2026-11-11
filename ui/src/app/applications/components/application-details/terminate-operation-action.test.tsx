import * as fs from 'fs';
import * as path from 'path';
import {ContextApis} from '../../../shared/context';
import * as models from '../../../shared/models';
import {services} from '../../../shared/services';

jest.mock('../../../shared/services', () => ({
    services: {
        applications: {
            terminateOperation: jest.fn(() => Promise.resolve(true))
        }
    }
}));

// The module is loaded lazily so the wiring check below reports its own failure when the Terminate entry is missing from the more menu.
const loadMenuItems = () => require('./terminate-operation-action').terminateOperationMenuItems;

const makeApp = (phase?: string) =>
    ({
        metadata: {name: 'test-app', namespace: 'argocd'},
        status: phase ? {operationState: {phase}} : {}
    }) as unknown as models.Application;

const makeContext = (confirmed: boolean) =>
    ({
        popup: {confirm: jest.fn(() => Promise.resolve(confirmed))},
        notifications: {show: jest.fn()}
    }) as unknown as ContextApis;

describe('application more menu wiring', () => {
    it('adds the Terminate action to the application more menu', () => {
        const source = fs.readFileSync(path.join(__dirname, 'application-details.tsx'), 'utf8');
        expect(source).toContain("terminateOperationMenuItems(app, appContext, <ActionMenuItem actionLabel='Terminate' />)");
    });
});

describe('terminateOperationMenuItems', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    it('offers the Terminate action only while the operation is Running', () => {
        const terminateOperationMenuItems = loadMenuItems();
        const ctx = makeContext(true);
        expect(terminateOperationMenuItems(makeApp('Running'), ctx, 'Terminate')).toHaveLength(1);
        expect(terminateOperationMenuItems(makeApp('Succeeded'), ctx, 'Terminate')).toHaveLength(0);
        expect(terminateOperationMenuItems(makeApp(), ctx, 'Terminate')).toHaveLength(0);
    });

    it('terminates the operation with the app name and namespace after confirmation', async () => {
        const terminateOperationMenuItems = loadMenuItems();
        const ctx = makeContext(true);
        const [item] = terminateOperationMenuItems(makeApp('Running'), ctx, 'Terminate');
        await item.action();
        expect(ctx.popup.confirm).toHaveBeenCalledWith('Terminate operation', 'Are you sure you want to terminate operation?');
        expect(services.applications.terminateOperation).toHaveBeenCalledWith('test-app', 'argocd');
    });

    it('does nothing when the confirmation is declined', async () => {
        const terminateOperationMenuItems = loadMenuItems();
        const ctx = makeContext(false);
        const [item] = terminateOperationMenuItems(makeApp('Running'), ctx, 'Terminate');
        await item.action();
        expect(services.applications.terminateOperation).not.toHaveBeenCalled();
    });

    it('shows an error notification when termination fails', async () => {
        const terminateOperationMenuItems = loadMenuItems();
        (services.applications.terminateOperation as jest.Mock).mockImplementationOnce(() => Promise.reject(new Error('boom')));
        const ctx = makeContext(true);
        const [item] = terminateOperationMenuItems(makeApp('Running'), ctx, 'Terminate');
        await item.action();
        expect(ctx.notifications.show).toHaveBeenCalledWith(expect.objectContaining({type: expect.anything()}));
    });
});
