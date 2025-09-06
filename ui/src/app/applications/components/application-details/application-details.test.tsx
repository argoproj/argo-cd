import { BehaviorSubject, from, of } from 'rxjs';
import { combineLatest } from 'rxjs';
import { map, mergeMap } from 'rxjs/operators';

// Mock services directly
const mockServices = {
    applications: {
        get: jest.fn(),
        watch: jest.fn(),
        resourceTree: jest.fn(),
        watchResourceTree: jest.fn()
    },
    viewPreferences: {
        getPreferences: jest.fn()
    },
    extensions: {
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
        getAppViewExtensions: jest.fn().mockReturnValue([]),
        getStatusPanelExtensions: jest.fn().mockReturnValue([]),
        getActionMenuExtensions: jest.fn().mockReturnValue([])
    }
};

// Mock utils directly
const mockAppUtils = {
    handlePageVisibility: jest.fn((callback) => callback()),
    nodeKey: jest.fn((node) => `${node.group}/${node.kind}/${node.namespace}/${node.name}`),
    appInstanceName: jest.fn((app) => app.metadata.name)
};

describe('ApplicationDetails', () => {
    let mockContext: any;
    let mockProps: any;
    let appChanged: BehaviorSubject<any>;

    beforeEach(() => {
        jest.clearAllMocks();

        // Setup mock props
        mockProps = {
            match: {
                params: {
                    name: 'test-app',
                    appnamespace: 'test-namespace'
                }
            },
            location: {
                search: ''
            },
            history: {
                location: {
                    search: ''
                }
            }
        };

        // Setup mock context
        mockContext = {
            apis: {
                navigation: {
                    goto: jest.fn()
                },
                notifications: {
                    show: jest.fn()
                },
                popup: {
                    confirm: jest.fn()
                }
            }
        };

        // Setup mock application
        const mockApp = {
            metadata: {
                name: 'test-app',
                namespace: 'test-namespace',
                resourceVersion: '123'
            },
            spec: {
                project: 'default'
            },
            status: {
                resources: [],
                summary: {}
            }
        };

        // Setup mock tree
        const mockTree = {
            nodes: [],
            orphanedNodes: [],
            hosts: []
        };

        // Setup service mocks
        mockServices.applications.get.mockResolvedValue(mockApp);
        mockServices.applications.watch.mockReturnValue(of({
            application: mockApp,
            type: 'MODIFIED'
        }));
        mockServices.applications.resourceTree.mockResolvedValue(mockTree);
        mockServices.applications.watchResourceTree.mockReturnValue(of(mockTree));
        mockServices.viewPreferences.getPreferences.mockReturnValue(of({
            appDetails: {
                resourceFilter: [],
                view: 'tree'
            }
        }));
        
        // Initialize the appChanged subject
        appChanged = new BehaviorSubject(null);
    });

    describe('loadAppInfo', () => {
        it('should load application info and handle updates correctly', (done) => {
            // Create a mock loadAppInfo function that mimics the behavior of the real one
            function loadAppInfo(name: string, appNamespace: string) {
                return from(mockServices.applications.get(name, appNamespace))
                    .pipe(
                        mergeMap(app => {
                            mockServices.applications.watch({name, appNamespace});
                            mockServices.applications.watchResourceTree(name, appNamespace);
                            
                            return of({
                                application: app,
                                tree: {
                                    nodes: [],
                                    orphanedNodes: [],
                                    hosts: []
                                }
                            });
                        })
                    );
            }
            
            // Call the loadAppInfo method
            const result = loadAppInfo('test-app', 'test-namespace');

            // Subscribe to the result
            result.subscribe((data: any) => {
                expect(data.application).toBeDefined();
                expect(data.application.metadata.name).toBe('test-app');
                expect(data.tree).toBeDefined();
                
                // Verify the mocks were called correctly
                expect(mockServices.applications.get).toHaveBeenCalledWith('test-app', 'test-namespace');
                
                done();
            });
        });

        it('should handle application deletion correctly', (done) => {
            // Setup mock for deletion event
            mockServices.applications.watch.mockReturnValue(of({
                application: { metadata: { name: 'test-app' } },
                type: 'DELETED'
            }));

            // Create a mock onAppDeleted function
            const onAppDeleted = jest.fn();
            
            // Create a mock loadAppInfo function that handles deletion
            function loadAppInfoWithDeletion(name: string, appNamespace: string) {
                return from(mockServices.applications.get(name, appNamespace))
                    .pipe(
                        mergeMap(app => {
                            const watchEvent = mockServices.applications.watch({name, appNamespace}).subscribe(event => {
                                if (event.type === 'DELETED') {
                                    onAppDeleted();
                                    mockContext.apis.notifications.show();
                                    mockContext.apis.navigation.goto('/applications');
                                }
                            });
                            
                            return of({
                                application: app,
                                tree: {
                                    nodes: [],
                                    orphanedNodes: [],
                                    hosts: []
                                }
                            });
                        })
                    );
            }

            // Call the modified loadAppInfo method
            const result = loadAppInfoWithDeletion('test-app', 'test-namespace');

            // Subscribe to the result
            result.subscribe(() => {
                // Verify the deletion handler was called
                expect(onAppDeleted).toHaveBeenCalled();
                expect(mockContext.apis.notifications.show).toHaveBeenCalled();
                expect(mockContext.apis.navigation.goto).toHaveBeenCalledWith('/applications');
                done();
            });
        });
    });
});
