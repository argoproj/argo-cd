/* eslint-env jest */
import requests from './requests';
import {UserService} from './user-service';

jest.mock('./requests', () => ({
    __esModule: true,
    default: {get: jest.fn()}
}));

const mockGet = requests.get as unknown as jest.Mock;

describe('UserService permissionDenied', () => {
    const service = new UserService();
    const seen: boolean[] = [];

    beforeAll(() => {
        service.permissionDenied.subscribe(denied => seen.push(denied));
    });

    it('starts out not denied', () => {
        expect(seen).toEqual([false]);
    });

    it('stays quiet when the request succeeds', async () => {
        mockGet.mockImplementation(() => Promise.resolve({body: {loggedIn: true, username: 'alice'}}));
        await expect(service.get()).resolves.toEqual({loggedIn: true, username: 'alice'});
        expect(seen).toEqual([false]);
    });

    it('stays quiet for a failure that is not a refusal', async () => {
        mockGet.mockImplementation(() => Promise.reject({status: 401}));
        await expect(service.get()).rejects.toEqual({status: 401});
        expect(seen).toEqual([false]);
    });

    it('reports a 403 and still rejects so callers can handle it', async () => {
        mockGet.mockImplementation(() => Promise.reject({response: {status: 403}}));
        await expect(service.get()).rejects.toEqual({response: {status: 403}});
        expect(seen).toEqual([false, true]);
    });

    it('stays latched once the session has been refused', async () => {
        mockGet.mockImplementation(() => Promise.resolve({body: {loggedIn: true, username: 'alice'}}));
        await service.get();
        expect(seen).toEqual([false, true]);
    });
});
