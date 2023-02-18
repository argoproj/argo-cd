import {sinceSeconds} from "./since";

test('sinceSeconds', () => {
    const now = new Date(0);
    expect(sinceSeconds('min', now)).toBe(0);
    expect(sinceSeconds('1m', now)).toBe(0 - 60);
    expect(sinceSeconds('5m', now)).toBe(0 - 5 * 60);
    expect(sinceSeconds('30m', now)).toBe(0 - 30 * 60);
    expect(sinceSeconds('1h', now)).toBe(0 - 60 * 60);
    expect(sinceSeconds('4h', now)).toBe(0 - 4 * 60 * 60);
})