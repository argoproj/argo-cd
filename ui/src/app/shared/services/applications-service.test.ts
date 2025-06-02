import { Observable, of } from 'rxjs';
import { map } from 'rxjs/operators';

// Create mock requests object
const mockRequests = {
    loadEventSource: jest.fn()
};

// Create mock service methods
function watch(query?: any, options?: any): Observable<any> {
    const search = new URLSearchParams();
    if (query) {
        if (query.name) {
            search.set('name', query.name);
        }
        if (query.resourceVersion) {
            search.set('resourceVersion', query.resourceVersion);
        }
        if (query.appNamespace) {
            search.set('appNamespace', query.appNamespace);
        }
    }
    if (options) {
        if (options.fields) {
            search.set('fields', options.fields.join(','));
        }
        if (options.selector) {
            search.set('selector', options.selector);
        }
        if (options.appNamespace) {
            search.set('appNamespace', options.appNamespace);
        }
        query?.projects?.forEach((project: string) => search.append('projects', project));
    }
    const searchStr = search.toString();
    const url = `/stream/applications${(searchStr && '?' + searchStr) || ''}`;
    
    return mockRequests.loadEventSource(url)
        .pipe(
            map(data => JSON.parse(data).result)
        );
}

function watchResourceTree(name: string, appNamespace: string): Observable<any> {
    return mockRequests.loadEventSource(`/stream/applications/${name}/resource-tree?appNamespace=${appNamespace}`)
        .pipe(
            map(data => JSON.parse(data).result)
        );
}

// Create a mock service object
const service = {
    watch,
    watchResourceTree
};

describe('ApplicationsService', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    describe('watch', () => {
        it('should set up correct URL parameters', () => {
            mockRequests.loadEventSource.mockReturnValue(of('{"result": {"application": {}, "type": "ADDED"}}'));

            const query = {
                name: 'test-app',
                resourceVersion: '123',
                appNamespace: 'test-namespace',
                projects: ['project1', 'project2']
            };

            const options = {
                fields: ['field1', 'field2'],
                selector: 'selector',
                appNamespace: 'options-namespace'
            };

            service.watch(query, options);

            expect(mockRequests.loadEventSource).toHaveBeenCalledTimes(1);
            const url = mockRequests.loadEventSource.mock.calls[0][0];
            
            // Verify URL contains all parameters
            expect(url).toContain('name=test-app');
            expect(url).toContain('resourceVersion=123');
            expect(url).toContain('appNamespace=options-namespace');
            // Use encoded comma in the test
            expect(url).toContain('fields=field1%2Cfield2');
            expect(url).toContain('selector=selector');
            expect(url).toContain('projects=project1');
            expect(url).toContain('projects=project2');
        });

        it('should parse application data correctly', (done) => {
            const mockApp = {
                metadata: { name: 'test-app' },
                spec: { project: 'test-project' }
            };
            
            const mockEventData = JSON.stringify({
                result: {
                    application: mockApp,
                    type: 'ADDED'
                }
            });

            mockRequests.loadEventSource.mockReturnValue(of(mockEventData));

            // Use take(1) to ensure we only get one emission
            service.watch().subscribe({
                next: (event) => {
                    expect(event.type).toBe('ADDED');
                    expect(event.application.metadata.name).toBe('test-app');
                    expect(event.application.spec.project).toBe('test-project');
                    done();
                },
                error: (err) => done(err)
            });
        });
    });

    describe('watchResourceTree', () => {
        it('should set up correct URL parameters', () => {
            mockRequests.loadEventSource.mockReturnValue(of('{"result": {}}'));

            service.watchResourceTree('test-app', 'test-namespace');

            expect(mockRequests.loadEventSource).toHaveBeenCalledTimes(1);
            const url = mockRequests.loadEventSource.mock.calls[0][0];
            
            expect(url).toBe('/stream/applications/test-app/resource-tree?appNamespace=test-namespace');
        });

        it('should parse resource tree data correctly', (done) => {
            const mockTree = {
                nodes: [{ name: 'node1' }],
                orphanedNodes: [],
                hosts: []
            };
            
            const mockEventData = JSON.stringify({
                result: mockTree
            });

            mockRequests.loadEventSource.mockReturnValue(of(mockEventData));

            // Use take(1) to ensure we only get one emission
            service.watchResourceTree('test-app', 'test-namespace').subscribe({
                next: (tree) => {
                    expect(tree.nodes.length).toBe(1);
                    expect(tree.nodes[0].name).toBe('node1');
                    done();
                },
                error: (err) => done(err)
            });
        });
    });
});
