import { initVisibilityRecovery } from './requests';

describe('initVisibilityRecovery', () => {
    let mockGetEventSource: jest.Mock;
    let mockGetLastMessage: jest.Mock;
    let mockTriggerReconnect: jest.Mock;
    let mockEventSource: any;
    let addEventListenerSpy: jest.SpyInstance;
    let removeEventListenerSpy: jest.SpyInstance;
    let eventListener: EventListener;

    beforeEach(() => {
        mockEventSource = {
            readyState: 1, // OPEN
            close: jest.fn()
        };
        mockGetEventSource = jest.fn().mockReturnValue(mockEventSource);
        mockGetLastMessage = jest.fn().mockReturnValue(Date.now());
        mockTriggerReconnect = jest.fn();

        // Mock addEventListener using spyOn
        addEventListenerSpy = jest.spyOn(document, 'addEventListener').mockImplementation((event, handler) => {
            if (event === 'visibilitychange') {
                eventListener = handler as EventListener;
            }
        });
        removeEventListenerSpy = jest.spyOn(document, 'removeEventListener').mockImplementation();

        // Define visibilityState property to be mutable
        Object.defineProperty(document, 'visibilityState', {
            value: 'visible',
            writable: true,
            configurable: true
        });
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    it('should register visibilitychange listener', () => {
        initVisibilityRecovery(mockGetEventSource, mockGetLastMessage, mockTriggerReconnect);
        expect(document.addEventListener).toHaveBeenCalledWith('visibilitychange', expect.any(Function));
    });

    it('should return a cleanup function that removes the listener', () => {
        const cleanup = initVisibilityRecovery(mockGetEventSource, mockGetLastMessage, mockTriggerReconnect);
        cleanup();
        expect(document.removeEventListener).toHaveBeenCalledWith('visibilitychange', expect.any(Function));
    });

    it('should do nothing if visibilityState is hidden', () => {
        initVisibilityRecovery(mockGetEventSource, mockGetLastMessage, mockTriggerReconnect);
        
        // Change visibility to hidden
        Object.defineProperty(document, 'visibilityState', { value: 'hidden', writable: true });
        
        eventListener({} as Event);
        
        expect(mockTriggerReconnect).not.toHaveBeenCalled();
    });

    it('should trigger reconnect if readyState is not OPEN', () => {
        mockEventSource.readyState = 2; // CLOSED
        initVisibilityRecovery(mockGetEventSource, mockGetLastMessage, mockTriggerReconnect);
        
        Object.defineProperty(document, 'visibilityState', { value: 'visible', writable: true });
        eventListener({} as Event);
        
        expect(mockTriggerReconnect).toHaveBeenCalled();
    });

    it('should trigger reconnect if connection is stale (zombie connection)', () => {
        const ZOMBIE_THRESHOLD = 2 * 60 * 1000;
        const staleTime = Date.now() - ZOMBIE_THRESHOLD - 1000; // 2분 1초 전
        mockGetLastMessage.mockReturnValue(staleTime);
        
        initVisibilityRecovery(mockGetEventSource, mockGetLastMessage, mockTriggerReconnect);
        
        Object.defineProperty(document, 'visibilityState', { value: 'visible', writable: true });
        eventListener({} as Event);
        
        expect(mockTriggerReconnect).toHaveBeenCalled();
    });

    it('should NOT trigger reconnect if connection is healthy', () => {
        mockEventSource.readyState = 1; // OPEN
        mockGetLastMessage.mockReturnValue(Date.now()); // Fresh
        
        initVisibilityRecovery(mockGetEventSource, mockGetLastMessage, mockTriggerReconnect);
        
        Object.defineProperty(document, 'visibilityState', { value: 'visible', writable: true });
        eventListener({} as Event);
        
        expect(mockTriggerReconnect).not.toHaveBeenCalled();
    });

    it('should handle null event source gracefully', () => {
        mockGetEventSource.mockReturnValue(null);
        initVisibilityRecovery(mockGetEventSource, mockGetLastMessage, mockTriggerReconnect);
        
        Object.defineProperty(document, 'visibilityState', { value: 'visible', writable: true });
        eventListener({} as Event);
        
        expect(mockTriggerReconnect).not.toHaveBeenCalled();
    });
});
