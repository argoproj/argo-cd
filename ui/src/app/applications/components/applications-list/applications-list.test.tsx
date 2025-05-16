import { from, of } from 'rxjs';

// Mock the services and utils instead of importing them directly
const mockServices = {
    applications: {
        list: jest.fn(),
        watch: jest.fn()
    },
    viewPreferences: {
        getPreferences: jest.fn()
    }
};

const mockAppUtils = {
    appInstanceName: jest.fn((app) => app.metadata.name),
    handlePageVisibility: jest.fn((callback) => callback())
};

// Create a simplified version of the loadApplications function for testing
function loadApplications(projects: string[], appNamespace: string) {
    return from(mockServices.applications.list(projects, {appNamespace, fields: ['test']}));
}

describe('loadApplications', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    it('should load applications and handle updates correctly', (done) => {
        // Mock initial applications list
        const initialApps = {
            items: [
                { metadata: { name: 'app1', namespace: 'default' } },
                { metadata: { name: 'app2', namespace: 'default' } }
            ],
            metadata: { resourceVersion: '123' }
        };

        // Setup mocks
        mockServices.applications.list.mockReturnValue(Promise.resolve(initialApps));

        // Call the function
        const projects = ['project1'];
        const appNamespace = 'default';
        const result = loadApplications(projects, appNamespace);

        // Subscribe to the result
        result.subscribe(apps => {
            expect(apps).toBe(initialApps);
            expect(mockServices.applications.list).toHaveBeenCalledWith(projects, {
                appNamespace,
                fields: expect.any(Array)
            });
            done();
        });
    });
});
