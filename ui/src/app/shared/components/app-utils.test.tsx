import {getHealthStatusColor} from './app-utils';
import {ARGO_GRAY6_COLOR} from './colors';
import {COLORS} from './colors';
import {HealthStatuses} from '../models';

test('getHealthStatusColor maps every health status to its palette color', () => {
    expect(getHealthStatusColor(HealthStatuses.Healthy)).toBe(COLORS.health.healthy);
    expect(getHealthStatusColor(HealthStatuses.Suspended)).toBe(COLORS.health.suspended);
    expect(getHealthStatusColor(HealthStatuses.Degraded)).toBe(COLORS.health.degraded);
    expect(getHealthStatusColor(HealthStatuses.Progressing)).toBe(COLORS.health.progressing);
    expect(getHealthStatusColor(HealthStatuses.Missing)).toBe(COLORS.health.missing);
    // text-readable label gray instead of the pale palette gray
    expect(getHealthStatusColor(HealthStatuses.Unknown)).toBe(ARGO_GRAY6_COLOR);
});
